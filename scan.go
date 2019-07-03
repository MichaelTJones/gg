package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/MichaelTJones/lex"
	"github.com/cavaliercoder/go-cpio"
	"github.com/klauspost/compress/zstd"
)

/*
Go-Grep: scan any number of Go source code files, where scanning means passing each
through Go-language lexical analysis and reporting lines where selected classes of
tokens match a search pattern defined by a reguar expression.
*/

// matching
var regex *regexp.Regexp // pattern
var sign int             // literal sign
var vInt uint64          // literal value

func doScan() {
	if flag.NArg() < 2 {
		return
	}

	s := NewScan()

	// handle "all" flag first before subsequent upper-case anti-flags
	if strings.Contains(flag.Arg(0), "a") {
		C = true
		D = true
		I = true
		K = true
		N = true
		O = true
		P = true
		R = true
		S = true
		T = true
		V = true
	}
	// initialize token class inclusion flags
	for _, class := range flag.Arg(0) {
		switch class {
		case 'a':
			// already noted
		case 'c':
			C = true
		case 'C':
			C = false
		case 'd':
			D = true
		case 'D':
			D = false
		case 'i':
			I = true
		case 'I':
			I = false
		case 'k':
			K = true
		case 'K':
			K = false
		case 'n':
			N = true
		case 'N':
			N = false
		case 'o':
			O = true
		case 'O':
			O = false
		case 'p':
			P = true
		case 'P':
			P = false
		case 'r':
			R = true
		case 'R':
			R = false
		case 's':
			S = true
		case 'S':
			S = false
		case 't':
			T = true
		case 'T':
			T = false
		case 'v':
			V = true
		case 'V':
			V = false
		default:
			fmt.Fprintf(os.Stderr, "error: unrecognized token class '%c'\n", class)
		}
	}

	// initialize regular expression matcher
	var err error
	regex, err = regexp.Compile(flag.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	// initialize numeric value matcher
	if V && len(flag.Arg(1)) > 0 {
		n := flag.Arg(1)
		if n[0] == '-' {
			sign = -1
			n = n[1:]
		}
		vInt, err = strconv.ParseUint(n, 0, 64)
		if err != nil {
			V = false
			// fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}

	println("scan begins")
	scanned := false

	// scan files in the file of filenames indicated by the "-list" option.
	if *flagList != "" {
		println("processing files listed in the -list option")
		s.List(*flagList)
		scanned = true
	}

	// scan files named on command line.
	if flag.NArg() > 0 {
		println("processing files listed on command line")
		for _, v := range flag.Args()[1:] {
			s.File(v)
		}
		scanned = true
	}

	// scan files named in standard input if nothing scanned yet.
	if !scanned {
		println("processing files listed in standard input")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			s.File(scanner.Text())
		}
	}
	s.Complete()
	println("scan ends")

	// generate output
	// if scanned && *flagCPUs != 1 {
	println("report begins")
	s.Report()
	println("report ends")
	// }
}

type Scan struct {
	complete bool
	path     string
	match    []string
	combined []*Scan
}

func NewScan() *Scan {
	return &Scan{}
}

func visible(name string) bool {
	if *flagVisible {
		for _, s := range strings.Split(name, string(os.PathSeparator)) {
			if s != "" && s != "." && s != ".." && s[0] == '.' {
				return false
			}
		}
	}
	return true
}

func isCompressed(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".bz2" || ext == ".gz" || ext == ".zst"
}

func decompress(oldName string, oldData []byte) (newName string, newData []byte, err error) {
	ext := filepath.Ext(oldName)
	if (ext == ".go" && len(oldData) > 0) || (ext == ".zip") {
		return oldName, oldData, nil // nothing to do
	}

	var oldSize int64
	var encoded, decoder io.Reader

	// Select source of encoded data
	switch {
	case len(oldData) == 0:
		// Read from named file
		file, err := os.Open(oldName)
		if err != nil {
			println(err)
			return oldName, nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			println(err)
			return oldName, nil, err
		}
		oldSize = info.Size()
		encoded = file
	default:
		// Use provided data (likely reading from an archive)
		oldSize = int64(len(oldData))
		encoded = bytes.NewReader(oldData)
	}

	// Select decompression algorithm based on file extension
	switch {
	case ext == ".bz2":
		decoder, err = bzip2.NewReader(encoded), nil
	case ext == ".gz":
		decoder, err = gzip.NewReader(encoded)
	case ext == ".zst":
		decoder, err = zstd.NewReader(encoded)
	default:
		decoder, err = encoded, nil // "just reading" is minimal compression
	}
	if err != nil {
		println(err) // error creating the decoder
		return oldName, nil, err
	}

	// Decompress the data
	if newData, err = ioutil.ReadAll(decoder); err != nil {
		println(err) // error using the decoder
		return oldName, nil, err
	}
	if ext != ".go" {
		// Decompress the name ("sample.go.zst" → "sample.go")
		newName = strings.TrimSuffix(oldName, ext)
		printf("  %8d → %8d bytes (%6.3f×)  decompress and scan %s",
			oldSize, len(newData), float64(len(newData))/float64(oldSize), oldName)
	} else {
		newName = oldName
		printf("  %8d bytes  scan %s", len(newData), oldName)
	}

	return newName, newData, nil
}

func isArchive(name string) bool {
	if isCompressed(name) {
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext) // unwrap the compression suffix
	}
	ext := filepath.Ext(name)
	return ext == ".cpio" || ext == ".tar" || ext == ".zip"
}

func isGo(name string) bool {
	if !*flagGo {
		return true
	}
	if isCompressed(name) {
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext) // unwrap the compression suffix
	}
	return filepath.Ext(name) == ".go"
}

func (s *Scan) List(name string) {
	file, err := os.Open(name)
	if err != nil {
		println(err)
		return
	}

	println("scanning list of files:", name)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s.File(scanner.Text())
	}
	file.Close()
}

func (s *Scan) File(name string) {
	if !visible(name) {
		return
	}

	info, err := os.Lstat(name)
	if err != nil {
		println(err)
		return
	}

	// process plain files
	if info.Mode().IsRegular() {
		var err error
		var data []byte
		if isArchive(name) && isCompressed(name) {
			name, data, err = decompress(name, nil)
			if err != nil {
				println(err)
				return
			}
		}

		var archive io.Reader
		switch {
		case len(data) == 0:
			f, err := os.Open(name)
			if err != nil {
				println(err)
				return
			}
			defer f.Close()
			archive = f
		default:
			archive = bytes.NewReader(data)
		}

		ext := strings.ToLower(filepath.Ext(name))
		switch {
		case ext == ".cpio":
			println("processing cpio archive", name)
			r := cpio.NewReader(archive)
			for {
				hdr, err := r.Next()
				if err == io.EOF {
					break // End of archive
				}
				if err != nil {
					println(err)
					return
				}
				memberName := name + "::" + hdr.Name // "archive.cpio::file.go"
				if !isGo(hdr.Name) {
					println("skipping file with unrecognized extension:", memberName)
					continue
				}
				bytes, err := ioutil.ReadAll(r)
				if err != nil {
					println(err)
					return
				}
				s.Scan(memberName, bytes)
			}
		case ext == ".tar":
			println("processing tar archive", name)
			tr := tar.NewReader(archive)
			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break // End of archive
				}
				if err != nil {
					println(err)
					return
				}
				memberName := name + "::" + hdr.Name // "archive.tar::file.go"
				if !isGo(hdr.Name) {
					println("skipping file with unrecognized extension:", memberName)
					continue
				}
				bytes, err := ioutil.ReadAll(tr)
				if err != nil {
					println(err)
					return
				}
				s.Scan(memberName, bytes)
			}
		case ext == ".zip":
			println("processing zip archive:", name)
			r, err := zip.OpenReader(name)
			if err != nil {
				println(err)
				return
			}
			defer r.Close()

			for _, f := range r.File {
				fullName := name + "::" + f.Name // "archive.zip::file.go"
				if !isGo(f.Name) {
					println("skipping file with unrecognized extension:", fullName)
					continue
				}
				rc, err := f.Open()
				if err != nil {
					println(err)
					return
				}
				bytes, err := ioutil.ReadAll(rc)
				rc.Close()
				if err != nil {
					println(err)
					return
				}
				s.Scan(fullName, bytes)
			}
		case isGo(name):
			s.Scan(name, nil)
		default:
			println("skipping file with unrecognized extension:", name)
		}
	} else if info.Mode().IsDir() { // process directories
		switch *flagRecursive {
		case false:
			// process files in this directory
			println("processing Go files in directory", name)

			bases, err := ioutil.ReadDir(name)
			if err != nil {
				println(err)
				return
			}
			for _, base := range bases {
				fullName := filepath.Join(name, base.Name())
				if visible(fullName) && isGo(fullName) {
					s.Scan(fullName, nil)
				}

			}
		case true:
			// process files in this directory hierarchy
			println("processing Go files in and under directory", name)

			walker := func(path string, info os.FileInfo, err error) error {
				if err != nil {
					println(err)
					return err
				}
				name := info.Name()
				if info.IsDir() {
					if !visible(name) {
						println("skipping hidden directory", name)
						return filepath.SkipDir
					}
				} else {
					if visible(path) && isGo(path) {
						s.Scan(path, nil)
					}
				}
				return nil
			}

			err = filepath.Walk(name, walker) // standard library walker
			if err != nil {
				println(err)
			}
		}
	}
}

type Work struct {
	name   string
	source []byte
}

var first bool = true
var workers int
var work chan Work
var result chan *Scan

func worker() {
	s := NewScan()
	for w := range work {
		s.scan(w.name, w.source)
	}
	result <- s
}

func (s *Scan) Scan(name string, source []byte) {
	if first {
		if *flagCPUs != 1 {
			workers = *flagCPUs
			work = make(chan Work, 32*workers)
			result = make(chan *Scan)
			for i := 0; i < workers; i++ {
				go worker()
			}
		}
		first = false
	}

	switch {
	case name != "": // another file to scan
		switch *flagCPUs {
		case 1:
			s.scan(name, source) // synchronous...wait for scan to complete
		default:
			work <- Work{name: name, source: source} // enqueue scan request
		}
	case name == "": // end of scan
		if *flagCPUs != 1 && workers != 0 {
			close(work) // request results
			for i := 0; i < workers; i++ {
				s.Combine(<-result) // combine results
			}
			close(result)
			workers = 0
		}
	}
}

func (s *Scan) scan(name string, source []byte) {
	var err error
	var newName string
	newName, source, err = decompress(name, source)
	if err != nil {
		return
	}

	s.path = newName
	line := -1
	lexer := &lex.Lexer{Input: string(source), Mode: lex.ScanGo} // | lex.SkipSpace}
	expectPackageName := false

	// Perform the scan by tabulating token types, subtypes, and values
	for tok, text := lexer.Scan(); tok != lex.EOF; tok, text = lexer.Scan() {
		// go mini-parser: expect package name after "package" keyword
		if expectPackageName && tok == lex.Identifier {
			if P && regex.MatchString(text) {
				s.match = append(s.match, lexer.GetLine())
			}
			expectPackageName = false
		} else if tok == lex.Keyword && text == "package" {
			expectPackageName = true // set expectations
		}

		handle := func(flag bool) {
			if flag && lexer.Line > line {
				if lexer.Type == lex.String && lexer.Subtype == lex.Raw {
					// match each line of the raw string individually
					scanner := bufio.NewScanner(strings.NewReader(text))
					lineInString := 0
					for scanner.Scan() {
						if regex.MatchString(scanner.Text()) {
							s.match = append(s.match, scanner.Text()+"\n")
							line = lexer.Line + lineInString
							lineInString++
						}
					}
				} else if regex.MatchString(text) {
					// match the token but print the line that contains it
					s.match = append(s.match, lexer.GetLine())
					line = lexer.Line
				}
			}
		}

		switch tok {
		case lex.Comment:
			handle(C)
		case lex.String:
			handle(S)
		case lex.Operator:
			handle(O)
		case lex.Rune:
			handle(R)
		case lex.Identifier:
			handle(I)
		case lex.Number:
			handle(N) // literal match

			// introducing... the value match
			if V && lexer.Line > line {
				var nS int
				var nI uint64
				n := text
				if n[0] == '-' { // never used, but someday...
					nS = -1
					n = n[1:]
				}
				nI, err = strconv.ParseUint(n, 0, 64)
				if err == nil && nS == sign && nI == vInt {
					s.match = append(s.match, lexer.GetLine()) // match the token but print the line
					line = lexer.Line
				}
			}
		case lex.Keyword:
			handle(K)
		case lex.Type:
			handle(T)
		case lex.Other:
			handle(D)
		case lex.Character:
			// seems maningless match unexpected illegal characters, maybe "."?
		}
	}
}

func (s *Scan) Combine(c *Scan) {
	s.combined = append(s.combined, c)
}

// Complete a scan
func (s *Scan) Complete() {
	// Completion is a one-time operation. if already done, it must not be done again.
	if s.complete {
		return
	}

	// Signal end of scan, await asynchronous completion, and combine all results.
	s.Scan("", nil)
	s.complete = true
}

func (s *Scan) Report() {
	// complete scan (if not already the case)
	if !s.complete {
		s.Complete()
	}

	// if *flagCPUs == 1 {
	// 	return // in single-cpu case the results are already reported
	// }

	file := os.Stdout
	switch lower := strings.ToLower(*flagOutput); {
	case lower == "[stdout]":
		file = os.Stdout
	case lower == "[stderr]":
		file = os.Stderr
	case lower != "":
		var err error
		file, err = os.Create(*flagOutput)
		if err != nil {
			println(err)
			return
		}
		defer file.Close()
	}

	// sort results by pathname
	s.combined = append(s.combined, s) // add ourselves to the list
	sort.Slice(s.combined, func(i, j int) bool {
		return s.combined[i].path < s.combined[j].path
	})

	w := bufio.NewWriter(file)
	// output matches
	for _, f := range s.combined { // for each file scanned
		for _, m := range f.match { // matches in line order
			w.WriteString(f.path)
			w.WriteString(": ")
			w.WriteString(m)
		}
	}
	w.Flush()
}
