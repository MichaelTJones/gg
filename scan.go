package main

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/MichaelTJones/lex"
	"github.com/klauspost/compress/zstd"
)

/*
Go-Grep: scan any number of Go source code files, where scanning means passing each
through Go-language lexical analysis and reporting lines where selected classes of
tokens match a search pattern defined by a reguar expression.
*/

// token class inclusion
// a: search all of the following
// c: search Comments ("//..." or "/*...*/")
// d: search Defined non-types (iota, nil, new, true,...)
// i: search Identifiers ([a-zA-Z][a-zA-Z0-9]*)
// k: search Keywords (if, for, func, go, ...)
// n: search Numbers as strings (255 as 255, 0.255, 1e255)
// o: search Operators (,+-*/[]{}()>>...)
// p: search Package names
// r: search Rune literals ('a', '\U00101234')
// s: search Strings ("quoted" or `raw`)
// t: search Types (bool, int, float64, map, ...)
// v: search numeric Values (255 as 0b1111_1111, 0377, 255, 0xff)
var G, C, D, I, K, N, O, P, R, S, T, V bool

// matching
var regex *regexp.Regexp // pattern

var sign int // literal sign
var vIsInt bool
var vInt uint64    // literal value
var vFloat float64 // literal value

func doScan() Summary {
	s := NewScan()
	fixedArgs := 2
	if *flagActLikeGrep {
		fixedArgs = 1
	}

	if flag.NArg() < fixedArgs {
		return Summary{}
	}

	// initialize regular expression matcher
	var err error
	regex, err = getRegexp(flag.Arg(fixedArgs - 1))
	if err != nil {
		return Summary{}
	}

	// gg mode
	mode := setupModeGG(flag.Args())
	C = mode.C
	D = mode.D
	G = mode.G
	I = mode.I
	K = mode.K
	N = mode.N
	O = mode.O
	P = mode.P
	R = mode.R
	S = mode.S
	T = mode.T
	V = mode.V
	vIsInt = mode.vIsInt
	vInt = mode.vInt
	vFloat = mode.vFloat

	println("scan begins")
	scanned := false

	// scan files in the file of filenames indicated by the "-list" option.
	if *flagList != "" {
		println("processing files listed in the -list option")
		*flagFileName = true // presume multiple files...print names
		s.List(*flagList)
		scanned = true
	}

	// scan files named on command line.
	if flag.NArg() > fixedArgs {
		println("processing files listed on command line")
		if flag.NArg() > fixedArgs+1 {
			*flagFileName = true // multiple files...print names
		}
		for _, v := range flag.Args()[fixedArgs:] {
			s.File(v)
		}
		scanned = true
	}

	// scan files named in standard input if nothing scanned yet.
	if !scanned {
		println("processing files listed in standard input")
		*flagFileName = true // multiple files...print names
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			s.File(scanner.Text())
		}
	}
	summary := s.Complete() // parallel rendevousz here...will wait
	println("scan ends")
	return summary
}

type Scan struct {
	path  string
	line  []uint32
	match []string

	bytes   int
	tokens  int
	lines   int
	matches int

	complete bool
	total    Summary
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
		processRegularFile(name, s)
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
type Summary struct {
	bytes   int
	tokens  int
	matches int
	lines   int
	files   int
}

var first = true
var workers int
var scattered int
var work []chan Work
var result []chan *Scan
var done chan Summary

func worker(index int) {
	for w := range work[index] {
		s := NewScan()
		s.scan(w.name, w.source)
		result[index] <- s
	}
	result[index] <- &Scan{complete: true} // signal that this worker is done
}

func (s *Scan) Scan(name string, source []byte) {
	if first {
		workers = *flagCPUs
		work = make([]chan Work, workers)
		result = make([]chan *Scan, workers)
		for i := 0; i < workers; i++ {
			work[i] = make(chan Work, 512)
			result[i] = make(chan *Scan, 512)
			go worker(i)
		}
		done = make(chan Summary)
		go reporter() // wait for and gather results
		first = false
	}

	switch {
	case name != "": // another file to scan
		work[scattered%workers] <- Work{name: name, source: source} // enqueue scan request
		scattered++
	case name == "": // end of scan
		for i := range work {
			close(work[i]) // signal completion to workers
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
	s.bytes += len(source)

	// handle grep mode
	if *flagActLikeGrep || G {
		scanner := bufio.NewScanner(bytes.NewReader(source))
		line := uint32(1)
		for scanner.Scan() {
			s.lines++
			if regex.MatchString(scanner.Text()) {
				s.match = append(s.match, scanner.Text()+"\n")
				s.matches++
				if *flagLineNumber {
					s.line = append(s.line, line)
				}
			}
			line++
		}
		return
	}

	// Perform the scan by tabulating token types, subtypes, and values
	line := -1
	lexer := &lex.Lexer{Input: string(source), Mode: lex.ScanGo} // | lex.SkipSpace}

	expectPackageName := false
	skip := false
	// theWholeLine := ""
	for tok, text := lexer.Scan(); tok != lex.EOF; tok, text = lexer.Scan() {
		s.tokens++

		// if !skip {
		// 	theWholeLine = lexer.GetLine()
		// 	if !regex.MatchString(theWholeLine) {
		// 		skip = true
		// 	}
		// }

		// go mini-parser: expect package name after "package" keyword
		if expectPackageName && tok == lex.Identifier {
			if P && regex.MatchString(text) {
				s.match = append(s.match, lexer.GetLine())
				// s.match = append(s.match, theWholeLine)
				s.matches++
				if *flagLineNumber {
					s.line = append(s.line, uint32(lexer.Line))
				}
			}
			expectPackageName = false
		} else if tok == lex.Keyword && text == "package" {
			expectPackageName = true // set expectations
		}

		handle := func(flag bool) {
			// if !skip {
			if true || !skip {
				if flag && lexer.Line > line {
					if lexer.Type == lex.String && lexer.Subtype == lex.Raw {
						// match each line of the raw string individually
						scanner := bufio.NewScanner(strings.NewReader(text))
						lineInString := 0
						for scanner.Scan() {
							if regex.MatchString(scanner.Text()) {
								s.match = append(s.match, scanner.Text()+"\n")
								s.matches++
								line = lexer.Line + lineInString
								lineInString++
								if *flagLineNumber {
									s.line = append(s.line, uint32(line+1))
								}
							}
						}
					} else if regex.MatchString(text) {
						// match the token but print the line that contains it
						s.match = append(s.match, lexer.GetLine())
						// s.match = append(s.match, theWholeLine)
						s.matches++
						line = lexer.Line
						if *flagLineNumber {
							s.line = append(s.line, uint32(line+1))
						}
					}
				}
			}
		}

		switch tok {
		case lex.Space:
			if text == "\n" {
				skip = false
				s.lines++
			}
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
				n := text
				var nS int
				if n[0] == '-' { // never used, but someday...
					nS = -1
					n = n[1:]
				}
				switch vIsInt {
				case true:
					var nI uint64
					nI, err = strconv.ParseUint(n, 0, 64)
					if err == nil && nS == sign && nI == vInt {
						s.match = append(s.match, lexer.GetLine()) // match the token but print the line
						line = lexer.Line
					}
				case false:
					var nF float64
					nF, err = strconv.ParseFloat(n, 64)
					if err == nil && nS == sign && nF == vFloat {
						s.match = append(s.match, lexer.GetLine()) // match the token but print the line
						line = lexer.Line
					}
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

// Complete a scan
func (s *Scan) Complete() Summary {
	if !s.complete {
		s.Scan("", nil)  // Signal end of additional files...
		s.total = <-done // ...and await completion.of scanning

		for i := range result {
			close(result[i])
		}

		s.complete = true // Record completion
	}
	return s.total
}

func reporter() {
	var w io.Writer

	switch lower := strings.ToLower(*flagOutput); {
	case lower == "" || lower == "[stdout]":
		file := os.Stdout
		if *flagBufferWrites {
			b := bufio.NewWriterSize(file, *flagBufferSize) // ensure buffered writes
			defer b.Flush()
			w = b
		} else {
			w = file
		}
	case lower == "[stderr]":
		file := os.Stderr
		if *flagBufferWrites {
			b := bufio.NewWriterSize(file, *flagBufferSize) // ensure buffered writes
			defer b.Flush()
			w = b
		} else {
			w = file
		}
	case lower != "":
		var err error
		file, err := os.Create(*flagOutput)
		if err != nil {
			println(err)
			return
		}
		defer file.Close()
		w = file
	}

	// summary statistics
	total := Summary{}

	// report results per file
	gathered := 0
	completed := 0
	for {
		// get next result in search order
		s := <-result[gathered%workers]
		gathered++

		// handle completion events
		if s.complete {
			completed++ // one more worker has finished
			if completed == workers {
				break // all workers have now finished
			}
			continue
		}

		// report this file's matching lines
		for i, m := range s.match {
			// first the filename, from "-h"
			if *flagFileName {
				fmt.Fprintf(w, "%s:", s.path)
			}

			// second the line number, from "-n"
			if *flagLineNumber {
				fmt.Fprintf(w, "%d:", s.line[i])
			}

			// finally, the match itself
			start := 0
			if *flagTrim {
				for start < len(m) {
					ch := m[start]
					if ch == ' ' || ch == '\t' {
						start++
					} else {
						break
					}
				}
				if start < len(m) {
					m = m[start:]
				}
			}
			fmt.Fprintf(w, "%s", m)
		}

		total.bytes += s.bytes
		total.tokens += s.tokens
		total.matches += s.matches
		total.lines += s.lines
		total.files++
	}

	// signal completion to main program
	done <- total // scanning complete, here are totals
}

func println(v ...interface{}) {
	if *flagLog != "" {
		log.Println(v...)
	}
}

func printf(f string, v ...interface{}) {
	if *flagLog != "" {
		log.Printf(f, v...)
	}
}

func plural(n int, fill string) string {
	if n == 1 {
		return fill
	}
	return "s"
}

type searchMode struct {
	// c: search Comments ("//..." or "/*...*/")
	C bool
	// d: search Defined non-types (iota, nil, new, true,...)
	D bool
	// grep mode ?
	G bool
	// i: search Identifiers ([a-zA-Z][a-zA-Z0-9]*)
	I bool
	// k: search Keywords (if, for, func, go, ...)
	K bool
	// n: search Numbers as strings (255 as 255, 0.255, 1e255)
	N bool
	// o: search Operators (,+-*/[]{}()>>...)
	O bool
	// p: search Package names
	P bool
	// r: search Rune literals ('a', '\U00101234')
	R bool
	// s: search Strings ("quoted" or `raw`)
	S bool
	// t: search Types (bool, int, float64, map, ...)
	T bool
	// v: search numeric Values (255 as 0b1111_1111, 0377, 255, 0xff)
	V      bool
	vIsInt bool
	vInt   uint64
	vFloat float64
}

func parseFirstArg(input string) searchMode {
	result := searchMode{}
	// a: search all of the following
	if strings.Contains(input, "a") {
		result.C = true
		result.D = true
		result.I = true
		result.K = true
		result.N = true
		result.O = true
		result.P = true
		result.R = true
		result.S = true
		result.T = true
		result.V = true
	}

	// initialize token class inclusion flags
	for _, class := range input {
		switch class {
		case 'a':
			// already noted
		case 'c':
			result.C = true
		case 'C':
			result.C = false
		case 'd':
			result.D = true
		case 'D':
			result.D = false
		case 'g':
			result.G = true
		case 'i':
			result.I = true
		case 'I':
			result.I = false
		case 'k':
			result.K = true
		case 'K':
			result.K = false
		case 'n':
			result.N = true
		case 'N':
			result.N = false
		case 'o':
			result.O = true
		case 'O':
			result.O = false
		case 'p':
			result.P = true
		case 'P':
			result.P = false
		case 'r':
			result.R = true
		case 'R':
			result.R = false
		case 's':
			result.S = true
		case 'S':
			result.S = false
		case 't':
			result.T = true
		case 'T':
			result.T = false
		case 'v':
			result.V = true
		case 'V':
			result.V = false
		default:
			fmt.Fprintf(os.Stderr, "error: unrecognized token class '%c'\n", class)
		}
	}
	return result
}

func setupModeGG(args []string) searchMode {
	res := searchMode{}
	if !*flagActLikeGrep {
		if len(args) < 2 {
			// not enough args received, complete args with empty strings
			for i := len(args); i < 2; i++ {
				args = append(args, "")
			}
		}
		// handle "all" flag first before subsequent upper-case anti-flags
		res = parseFirstArg(args[0])

		// initialize numeric value matcher
		if res.V && len(args[1]) > 0 {
			n := args[1]
			if n[0] == '-' {
				sign = -1
				n = n[1:]
			}
			var err error
			res.vInt, err = strconv.ParseUint(n, 0, 64)
			res.vIsInt = true
			if err != nil {
				res.vIsInt = false
				// we did not consume all the input...maybe it is a float.
				res.vFloat, err = strconv.ParseFloat(n, 64)
				_ = res.vFloat + -5.25
				if err != nil {
					res.V = false
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
			}
		}
	}
	return res
}

func getRegexp(input string) (*regexp.Regexp, error) {
	regexp, err := regexp.Compile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	return regexp, err
}

// Scanner is an interace created to allow us to create some tests
type Scanner interface {
	Scan(name string, source []byte)
}

type ReadNexter interface {
	Read(p []byte) (n int, err error)
	Next() (string, error)
}

func processRegularFile(name string, s Scanner) {
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
		r := newMultiReader(archive, ext, "")
		scanFile(name, r, s)
	case ext == ".tar":
		println("processing tar archive", name)
		r := newMultiReader(archive, ext, "")
		scanFile(name, r, s)
	case ext == ".zip":
		println("processing zip archive:", name)
		mr := newMultiReader(nil, ext, name)
		scanFile(name, mr, s)
	case isGo(name):
		s.Scan(name, nil)
	default:
		println("skipping file with unrecognized extension:", name)
	}
}

func scanFile(fileName string, r ReadNexter, s Scanner) {
	for {
		name, err := r.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			println(err)
			return
		}

		memberName := fileName + "::" + name // "archive.cpio::file.go"
		if !isGo(name) {
			println("skipping file with unrecognized extension:", memberName)
			continue
		}
		var buf bytes.Buffer
		buf.ReadFrom(r)
		bytes := buf.Bytes()
		if err != nil {
			println(err)
			return
		}
		s.Scan(memberName, bytes)
	}
}

func getResourceUsage() (user, system float64, size uint64) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		println("Error: unable to gather resource usage data:", err)
	}
	user = float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6   // work by this process
	system = float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6 // work by OS on behalf of this process (reading files)
	size = uint64(uint32(usage.Maxrss))
	return
}
