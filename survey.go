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
	"sort"
	"strings"
	"time"

	"github.com/MichaelTJones/lex"
	"github.com/cavaliercoder/go-cpio"
	"github.com/klauspost/compress/zstd"
)

/*
Survey any number of Go source code files, where surveying means passing each through a
Go-language lexical analysis and tabulating the various tokens found: the number of each
keyword and pre-defined identifier, the number and type of comments and strings, the
number and value of identifiers, the frequency of operators, and so on. The result is a
series of tables detailing counts and values.

Some of the counts have structural meaning: the number of "package" statements is also the
number of Go-language source files and the numbers of left and right parenthesis,
brackets, and braces must match in valid Go code. When surveying the Go source tree, for
example, the count of "./go/src" shows these values to balance as expected, while the full
survey of "./go" -- which includes "./go/test" -- does not match because of tests that
have invalid syntax as the purpose of their test. These files are easily identified with
the "-v" verbose option. In verbose mode, the report and any log file show the out of
balance files. Here is the result for the Go 1.13 source tree:

	(2:1) [0:0] {1:0}  go/test/fixedbugs/bug435.go
	(2:1) [0:0] {1:1}  go/test/fixedbugs/issue13248.go
	(1:1) [0:0] {1:0}  go/test/fixedbugs/issue13274.go
	(6:4) [0:0] {2:2}  go/test/fixedbugs/issue13319.go
	(1:0) [0:0] {0:0}  go/test/fixedbugs/issue15611.go
	(1:2) [0:0] {2:2}  go/test/fixedbugs/issue17328.go
	(1:1) [0:0] {3:2}  go/test/fixedbugs/issue18092.go
	(2:1) [0:0] {1:1}  go/test/fixedbugs/issue19667.go
	(1:1) [2:0] {1:0}  go/test/fixedbugs/issue20789.go
	(8:6) [1:1] {5:4}  go/test/fixedbugs/issue22164.go
	(7:10) [0:3] {7:7}  go/test/fixedbugs/issue22581.go
	(1:1) [0:0] {2:0}  go/test/syntax/semi1.go
	(1:1) [0:0] {2:0}  go/test/syntax/semi2.go
	(1:1) [0:0] {2:0}  go/test/syntax/semi3.go
	(1:1) [0:0] {2:0}  go/test/syntax/semi4.go
	(1:1) [0:0] {1:0}  go/test/syntax/semi5.go
	(1:1) [1:1] {2:1}  go/test/syntax/vareq.go

The meaning of the first line is that "go/test/fixedbugs/bug435.go" has 2 "(" and 1 ")",
no "[" or "]", and 1 "{" and 0 "}". These mismatches, result in an overall summary
mismatch:

	Count    Percent  Token subtype
	981939   18.8815%  ,
	602038   11.5765%  (
	602034   11.5764%  )
	601369   11.5636%  .
	463464    8.9119%  =
	422346    8.1212%  :
	380508    7.3167%  {
	380493    7.3164%  }
	152008    2.9229%  ]
	152007    2.9229%  [
*/

func doSurvey() {
	if *flagVerbose {
		detailCPU() // useful in benchmark analysis
	}

	println("survey begins")
	s := NewSurvey()
	surveyed := false

	// survey files in the file of filenames indicated by the "-list" option.
	if *flagList != "" {
		println("processing files listed in the -list option")
		s.List(*flagList)
		surveyed = true
	}

	// survey files named on command line.
	if flag.NArg() != 0 {
		println("processing files listed on command line")
		for _, v := range flag.Args() {
			s.File(v)
		}
		surveyed = true
	}

	// survey files named in standard input if nothing surveyed yet.
	if !surveyed {
		println("processing files listed in standard input")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			s.File(scanner.Text())
		}
	}
	s.Complete()
	println("survey ends")

	// generate output report
	if surveyed {
		println("report begins")
		s.Report()
		println("report ends")
	}
}

type Survey struct {
	complete bool // post-survey data adjustments have been made
	start    time.Time
	elapsed  float64 // wall-clock time for survey
	user     float64 // user time in process, sum of all threads
	system   float64 // system time on process' behalf
	size     uint64  // process RSS in bytes

	good []string // go files that had no parsing issues
	bad  []string // go files that were strange in some way

	files  int
	lines  int
	bytes  int
	tokens int

	extensions map[string]int // file extensions processed: .go, .zip, .gz, ...

	ascii       map[string]int
	bases       map[string]int
	comments    map[string]int
	identifiers map[string]int
	keywords    map[string]int
	operators   map[string]int
	others      map[string]int
	packages    map[string]int
	runes       map[string]int
	strings     map[string]int
	types       map[string]int
	unicode     map[string]int

	countComments    [3]int // count directly rather than via strings-in-map
	countIdentifiers [3]int
	countStrings     [3]int
	countBases       [6]int
}

func NewSurvey() *Survey {
	return &Survey{
		extensions:  make(map[string]int),
		packages:    make(map[string]int),
		operators:   make(map[string]int),
		runes:       make(map[string]int),
		ascii:       make(map[string]int),
		unicode:     make(map[string]int),
		keywords:    make(map[string]int),
		types:       make(map[string]int),
		others:      make(map[string]int),
		comments:    make(map[string]int),
		identifiers: make(map[string]int),
		bases:       make(map[string]int),
		strings:     make(map[string]int),
	}
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
		printf("  %8d → %8d bytes (%6.3f×)  decompress and survey %s",
			oldSize, len(newData), float64(len(newData))/float64(oldSize), oldName)
	} else {
		newName = oldName
		printf("  %8d bytes  survey %s", len(newData), oldName)
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

func (s *Survey) List(name string) {
	file, err := os.Open(name)
	if err != nil {
		println(err)
		return
	}

	println("surveying list of files:", name)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s.File(scanner.Text())
	}
	file.Close()
}

func (s *Survey) File(name string) {
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
			s.extensions[filepath.Ext(name)]++
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
			s.extensions[filepath.Ext(name)]++
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
				s.Survey(memberName, bytes)
			}
		case ext == ".tar":
			println("processing tar archive", name)
			s.extensions[filepath.Ext(name)]++
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
				s.Survey(memberName, bytes)
			}
		case ext == ".zip":
			println("processing zip archive:", name)
			s.extensions[filepath.Ext(name)]++
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
				s.Survey(fullName, bytes)
			}
		case isGo(name):
			s.Survey(name, nil)
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
					s.Survey(fullName, nil)
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
						s.Survey(path, nil)
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
var result chan *Survey

func worker() {
	s := NewSurvey()
	for w := range work {
		s.survey(w.name, w.source)
	}
	result <- s
}

func (s *Survey) Survey(name string, source []byte) {
	if first {
		s.start = time.Now()

		if *flagCPUs != 1 {
			workers = *flagCPUs
			work = make(chan Work, 32*workers)
			result = make(chan *Survey)
			for i := 0; i < workers; i++ {
				go worker()
			}
		}
		first = false
	}

	switch {
	case name != "": // another file to survey
		switch *flagCPUs {
		case 1:
			s.survey(name, source) // synchronous...wait for survey to complete
		default:
			work <- Work{name: name, source: source} // enqueue survey request
		}
	case name == "": // end of survey
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

func (s *Survey) survey(name string, source []byte) {
	var err error
	var newName string
	if isCompressed(name) {
		s.extensions[filepath.Ext(name)]++
	}
	newName, source, err = decompress(name, source)
	if err != nil {
		return
	}
	// if newName != name ||{
	s.extensions[filepath.Ext(newName)]++
	// }

	lexer := &lex.Lexer{Input: string(source), Mode: lex.ScanGo | lex.SkipSpace}
	var c [256]int // used to count operator characters to detect imbalanced () {} []
	badChars := ""
	expectPackageName := false

	// Perform the survey by tabulating token types, subtypes, and values
	for tok, text := lexer.Scan(); tok != lex.EOF; tok, text = lexer.Scan() {
		s.tokens++

		// go mini-parser: expect package name after "package" keyword
		if expectPackageName && tok == lex.Identifier {
			s.packages[text]++
			expectPackageName = false
			continue
		}
		if tok == lex.Keyword && text == "package" {
			expectPackageName = true // set expectations
		}

		switch tok {
		case lex.Comment:
			s.countComments[lexer.Subtype]++
		case lex.String:
			s.countStrings[lexer.Subtype]++
		case lex.Operator:
			s.operators[text]++
			c[byte(text[0])]++ // count () [] {} (and every other single character)
		case lex.Rune:
			s.runes[text]++
		case lex.Identifier:
			s.countIdentifiers[lexer.Subtype]++ // ASCII-only or Unicode
			switch lexer.Subtype {
			case lex.ASCII:
				s.ascii[text]++
			case lex.Unicode:
				s.unicode[text]++
			}
		case lex.Number:
			// note: safe because lex.Octal means len(text) >= 2 ("00"..."07" are the shortest)
			if lexer.Subtype == lex.Octal && (text[1] != 'o' && text[1] != 'O') {
				s.countBases[5]++
			} else {
				s.countBases[lexer.Subtype]++
			}
		case lex.Keyword:
			s.keywords[text]++
		case lex.Type:
			s.types[text]++
		case lex.Other:
			s.others[text]++
		case lex.Character:
			badChars += text // only happens if go code won't compile because junk characters in file
		}
	}

	s.files++
	s.lines += bytes.Count(source, []byte{'\n'})
	s.bytes += len(source)

	good := true
	if c['('] != c[')'] || c['['] != c[']'] || c['{'] != c['}'] { // counts match except in compiler failure tests
		name += fmt.Sprintf(" «balance (%d:%d) [%d:%d] {%d:%d}»", c['('], c[')'], c['['], c[']'], c['{'], c['}'])
		good = false
	}
	if badChars != "" {
		name += fmt.Sprintf(" «unrecognized %q»", badChars)
		good = false
	}
	switch good {
	case true:
		s.good = append(s.good, name)
	case false:
		s.bad = append(s.bad, name)
	}
}

func (s *Survey) Combine(c *Survey) {
	s.elapsed += c.elapsed

	s.files += c.files
	s.lines += c.lines
	s.tokens += c.tokens
	s.bytes += c.bytes

	s.good = append(s.good, c.good...)
	s.bad = append(s.bad, c.bad...)

	// go2 dream: s.packages += c.packages
	combineMap(s.extensions, c.extensions)
	combineMap(s.packages, c.packages)
	combineMap(s.operators, c.operators)
	combineMap(s.runes, c.runes)
	combineMap(s.ascii, c.ascii)
	combineMap(s.unicode, c.unicode)
	combineMap(s.keywords, c.keywords)
	combineMap(s.types, c.types)
	combineMap(s.others, c.others)
	combineMap(s.comments, c.comments)
	combineMap(s.identifiers, c.identifiers)
	combineMap(s.bases, c.bases)
	combineMap(s.strings, c.strings)

	// go2 dream: s.countComments += c.countComments
	for i, v := range c.countComments {
		s.countComments[i] += v
	}
	for i, v := range c.countIdentifiers {
		s.countIdentifiers[i] += v
	}
	for i, v := range c.countStrings {
		s.countStrings[i] += v
	}
	for i, v := range c.countBases {
		s.countBases[i] += v
	}
}

func combineMap(s, c map[string]int) {
	for k, v := range c {
		s[k] += v
	}
}

// Complete a survey
func (s *Survey) Complete() {
	// Completion is a one-time operation. if already done, it must not be done again.
	if s.complete {
		return
	}

	// Signal end of survey, await asynchronous completion, and combine all results.
	s.Survey("", nil)

	// Populate map-from-counts as needed
	s.comments[`general (/*…*/)`] += s.countComments[1]
	s.comments[`line (//…'\n')`] += s.countComments[2]

	s.identifiers[`ASCII-only`] += s.countIdentifiers[1]
	s.identifiers[`Unicode`] += s.countIdentifiers[2]

	s.bases[`prefixed binary (/0[bB][0-1]+/)`] += s.countBases[1]
	s.bases[`prefixed octal (/0[oO][0-7]+/)`] += s.countBases[2]
	s.bases[`decimal (/0&#124;([1-9][0-9]*)/)`] += s.countBases[3]
	s.bases[`prefixed hexadecimal (/0[xX][0-9a-fA-F]+/)`] += s.countBases[4]
	s.bases[`legacy octal (/0[0-7]+/)`] += s.countBases[5]

	s.strings[`quoted ("…")`] += s.countStrings[1]
	s.strings["raw (`…`)"] += s.countStrings[2]

	// remove package name references from the general identifier list
	for p := range s.packages {
		delete(s.ascii, p)
		delete(s.unicode, p)
	}

	// stop the timer: don't charge future logging and reporting against survey speed
	s.elapsed = time.Since(s.start).Seconds()
	s.user, s.system, s.size = getResourceUsage()

	var exts []string
	for key := range s.extensions {
		exts = append(exts, `"`+key+`"`)
	}
	sort.Strings(exts)
	if len(exts) > 0 {
		println("filetypes processed:", strings.Join(exts, ", "))
	}

	sort.Strings(s.good)
	sort.Strings(s.bad)

	if *flagVerbose {
		// if len(s.good) > 0 {
		// 	detail("")
		// 	detail("files that passed the Go lexical scan:")
		// 	for _, v := range s.good {
		// 		detail("  good", v)
		// 	}
		// }
		if len(s.bad) > 0 {
			println("")
			println("files that failed the Go lexical scan:")
			for _, v := range s.bad {
				println("  bad", v)
			}
		}
	}

	// detail("survey complete")
	println("processed", len(s.good), "good and", len(s.bad), "bad files in", s.elapsed, "seconds")

	s.complete = true
}

func (s *Survey) Report() {
	// complete survey (if not already the case)
	if !s.complete {
		s.Complete()
	}

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

	s.reportProcessing(file, "Processing summary")

	if s.files == 0 || s.lines == 0 || s.bytes == 0 {
		return
	}

	if *flagVerbose && len(s.bad) > 0 {
		s.reportImbalance(file, "Problem files")
	}

	// usage of language features (0 means show all values)
	reportSurvey(file, "Comment style popularity", s.comments, 0)
	reportSurvey(file, "String style popularity", s.strings, 0)
	reportSurvey(file, "Numeric base popularity", s.bases, 0)
	reportSurvey(file, "Reserved keyword popularity", s.keywords, 0)
	reportSurvey(file, "Predefined type popularity", s.types, 0)
	reportSurvey(file, "Other predefined popularity", s.others, 0)
	reportSurvey(file, "Operator popularity", s.operators, 0)
	reportSurvey(file, "Identifier subtype", s.identifiers, 0)

	// developer chosen symbols (report limited to no more than *flagLines lines)
	reportSurvey(file, "Package name popularity", s.packages, *flagLines)
	reportSurvey(file, "ASCII Identifier popularity", s.ascii, *flagLines)
	reportSurvey(file, "Unicode Identifier popularity", s.unicode, *flagLines)
	reportSurvey(file, "Rune constant popularity", s.runes, *flagLines)
}

func (s *Survey) reportProcessing(file *os.File, title string) {
	if *flagStyle == "markdown" {
		fmt.Fprintf(file, "\n")
		fmt.Fprintf(file, "# Go survey  \n")
		fmt.Fprintf(file, "\n")

		fmt.Fprintf(file, "## *%s*  \n", title)
		fmt.Fprintf(file, "\n")
		fmt.Fprintf(file, "Count | Units | Detail  \n")
		fmt.Fprintf(file, "---:|---|---  \n")

		fmt.Fprintf(file, "%d | file%s | source code groups  \n", s.files, plural(s.files, ""))
		fmt.Fprintf(file, "%d | file%s | original source code files  \n", s.keywords["package"], plural(s.keywords["package"], ""))
		fmt.Fprintf(file, "%d | %s | %.0f per sec  \n", s.lines, "lines", float64(s.lines)/s.elapsed)
		fmt.Fprintf(file, "%d | %s | %.0f per sec  \n", s.tokens, "tokens", float64(s.tokens)/s.elapsed)
		fmt.Fprintf(file, "%d | %s | %.0f per sec  \n", s.bytes, "bytes", float64(s.bytes)/s.elapsed)

		fmt.Fprintf(file, "%.6f | seconds | read/lex/tabulate/sort/select  \n", s.elapsed)
		if s.elapsed > 0 && s.bytes != 0 && *flagCPUs > 1 {
			fmt.Fprintf(file, "%d | worker%s | (parallel speedup = %.2f x with SMT)  \n",
				*flagCPUs, plural(*flagCPUs, ""), (s.user+s.system)/s.elapsed)
		}
	} else {
		fmt.Fprintf(file, "  %s  %s %s\n", strings.Repeat("━", 70-len(title)-6), title, strings.Repeat("━", 2))

		fmt.Fprintf(file, "  %10d %-7s (%s)\n", s.files,
			fmt.Sprintf("file%s", plural(s.files, "")),
			"source code groups")
		fmt.Fprintf(file, "  %10d %-7s (%s)\n", s.keywords["package"],
			fmt.Sprintf("file%s", plural(s.keywords["package"], "")),
			"original source code files")
		fmt.Fprintf(file, "  %10d %-7s (%10.0f per sec)\n", s.lines, "lines", float64(s.lines)/s.elapsed)
		fmt.Fprintf(file, "  %10d %-7s (%10.0f per sec)\n", s.tokens, "tokens", float64(s.tokens)/s.elapsed)
		fmt.Fprintf(file, "  %10d %-7s (%10.0f per sec)\n", s.bytes, "bytes", float64(s.bytes)/s.elapsed)

		fmt.Fprintf(file, "  %10.6f seconds to read/lex/tabulate/sort/adjust  \n", s.elapsed)
		if s.elapsed > 0 && s.bytes != 0 && *flagCPUs > 1 {
			fmt.Fprintf(file, "  %10d worker%s (parallel speedup = %.2f x with SMT)\n",
				*flagCPUs, plural(*flagCPUs, ""), (s.user+s.system)/s.elapsed)
		}
		fmt.Fprintf(file, "  %s\n", strings.Repeat("─", 70))
		fmt.Fprintf(file, "\n")
	}
}

func (s *Survey) reportImbalance(file *os.File, title string) {
	if len(s.bad) == 0 {
		return
	}

	if *flagStyle == "markdown" {
		fmt.Fprintf(file, "\n")
		fmt.Fprintf(file, "## *%s*  \n", title)
		fmt.Fprintf(file, "\n")
		fmt.Fprintf(file, "Problem | File  \n")
		fmt.Fprintf(file, "---:|--- \n")

		for _, v := range s.bad {
			part := strings.SplitN(v, " ", 2)
			part[1] = strings.TrimLeft(part[1], "«")
			part[1] = strings.TrimRight(part[1], "»")
			t := strings.SplitN(part[1], " ", 2)
			fmt.Fprintf(file, "%s | %s \n", t[1], part[0])
		}
	} else {
		fmt.Fprintf(file, "  %s  %s %s\n", strings.Repeat("━", 70-len(title)-6), title, strings.Repeat("━", 2))
		for _, v := range s.bad {
			part := strings.SplitN(v, " ", 2)
			part[1] = strings.TrimLeft(part[1], "«")
			part[1] = strings.TrimRight(part[1], "»")
			t := strings.SplitN(part[1], " ", 2)
			fmt.Fprintf(file, "  %20s  %s \n", t[1], part[0])
		}
		fmt.Fprintf(file, "  %s\n", strings.Repeat("─", 70))
		fmt.Fprintf(file, "\n")
	}
}

func reportSurvey(file *os.File, title string, m map[string]int, n int) {
	// skip empty surveys
	if len(m) == 0 {
		return
	}

	type Pair struct {
		n int
		s string
	}

	// order data by usage
	p := make([]Pair, len(m))
	i := 0
	t := 0
	for s, n := range m {
		p[i] = Pair{n: n, s: s}
		i++
		t += n
	}
	sort.Slice(p, func(i int, j int) bool {
		if p[i].n != p[j].n {
			return p[i].n > p[j].n
		}
		return p[i].s < p[j].s
	})

	if *flagStyle == "markdown" {
		fmt.Fprintf(file, "\n")
		fmt.Fprintf(file, "## *%s*  \n", title)
		fmt.Fprintf(file, "\n")
		fmt.Fprintf(file, "Count | Frequency | Detail\n")
		fmt.Fprintf(file, "---:|---:|---\n")

		unique := 0
		subtotal := 0
		for i, v := range p {
			if v.n == 0 {
				continue
			}
			if n == 0 || i < n {
				escaped := v.s
				escaped = strings.ReplaceAll(escaped, "|", "&#124;")  // protect '|'
				escaped = strings.ReplaceAll(escaped, "`", "&grave;") // protect '`'
				fmt.Fprintf(file, "  %d | %.4f%% | %s  \n", v.n, (100*float64(v.n))/float64(t), escaped)
			} else {
				unique++
				subtotal += v.n
			}
		}
		if subtotal > 0 {
			fmt.Fprintf(file, "  %d | %.4f%% | (%d more with %d unique values)  \n", subtotal, (100*float64(subtotal))/float64(t), subtotal, unique)
		}
		fmt.Fprintf(file, "  %d | %.4f%% | %s  \n", t, 100.0, "total")
	} else {
		fmt.Fprintf(file, "  %s  %s %s\n", strings.Repeat("━", 70-len(title)-6), title, strings.Repeat("━", 2))
		fmt.Fprintf(file, "  %9s  %9s  %s\n", "Count", "Percent", "Token subtype")
		fmt.Fprintf(file, "  %s\n", strings.Repeat("─", 70))
		unique := 0
		subtotal := 0
		for i, v := range p {
			if v.n == 0 {
				continue
			}
			if n == 0 || i < n {
				fmt.Fprintf(file, "  %9d  %8.4f%%  %s\n", v.n, (100*float64(v.n))/float64(t), v.s)
			} else {
				unique++
				subtotal += v.n
			}
		}
		if subtotal > 0 {
			fmt.Fprintf(file, "  %9d  %8.4f%%  (%d more with %d unique values)\n", subtotal, (100*float64(subtotal))/float64(t), subtotal, unique)
		}
		fmt.Fprintf(file, "  %s\n", strings.Repeat("─", 70))
		fmt.Fprintf(file, "  %9d  %8.4f%%  %s\n", t, 100.0, "total")
		fmt.Fprintf(file, "\n\n")
	}
}
