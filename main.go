package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

// Survey-related flags
var flagCPUs = flag.Int("cpu", 0, "number of CPUs to use (0 for all)")
var flagGo = flag.Bool("go", true, `limit survey to Go files ("main.go")`)
var flagLines = flag.Int("lines", 32, "maximum number of lines in report categories")
var flagList = flag.String("list", "", "list of filenames to survey")
var flagLog = flag.String("log", "", `write log to named file (or "[stdout]" or "[stderr]")`)
var flagOutput = flag.String("o", "", `write output to named file (or "[stdout]" or "[stderr]")`)
var flagRecursive = flag.Bool("r", false, "survey directories recursively")
var flagStyle = flag.String("style", "plain", "output style: plain, markdown")
var flagVerbose = flag.Bool("v", false, "verbose logging and reporting")
var flagVisible = flag.Bool("visible", true, `limit survey to visible files (skip ".hidden.go")`)

// Split-related flags (plus flagOutput)
var flagSize = flag.Int("size", 0, "minimum byte size for parts of split files")
var flagSplit = flag.String("split", "", "split named file")

var usage = `Survey gathers and reports summary statistics of Go code.

Surveys files listed by name as command line arguments, in a list of filenames provided
the "-list" argument, or if neither is provided, reads filenames from the standard input.
This last is useful in shell pipelines such as "find . -name "*.go" | survey"

Files may be either Go source files or directories. Source files include typical ".go"
files; compressed ".go" files named ".go.bz2", ".go.gz", or ".go.zst" for Bzip2, Gzip, and
ZStandard compression formats; archives of any such files in the formats "a.cpio",
"a.tar", or "a.zip"; or, finally, compressed archives as in "a.cpio.bz2" and "a.tar.gz".
If a named file is a directory then all Go source files in that directory are surveyed
without visiting subdirectories. With the "-r" flag enabled, named directories are
processed RECURSIVELY, finding and surveying each Go source file in that directory's
hierarchy.

By default, the directory traversal and file surveying logic ignore directories and files
with an initial period in their basenames following UNIX shell tradition. The VISIBLE
option "-visible" may be used to include hidden files and directories in the survey.

The "-v" VERBOSE argument requests details of file processing and filesystem traversal,
reporting files with unbalanced "()[]{}" that should appear in pairs, files with improper
Unicode characters, and other curiosities. Since such code will not compile, these files
are usually tests. When in verbose mode the report will list any problem files with a
summary of the problem. The format is quoted strings for unexpected characters ("@#") and
a token balance count for mismatched Go operators. When you see:

  (7:10) [0:3] {7:7}  /Users/mtj/go/test/fixedbugs/issue22581.go

the (7:10) means that the named file has 7 left parenthesis and 10 right ones, [0:3] means
zero left and 3 right square brackets, and {7:7} shows 7 matching left and right braces.
Each number is beside the symbol it counts.

Output STYLE is set with the "-style" option. The default value "plain" prints data
simply; with the value "markdown" or "md" it prepares output for stylized display in
tables such as:

   https://gist.github.com/MichaelTJones/ca0fd339401ebbe79b9cbb5044afcfe2 (Go 1.13)
   https://gist.github.com/MichaelTJones/609589e05017da4be52bc2810e9df4e8 (Go Corpus 0.01)

Details of processing errors, such as file access problems, are available by setting the
LOGGING filename with the "-log" option. Two special log filenames are recognized:
"[stdout]" and "[stderr]" and route logging output to these standard UNIX destinations.
Without logging, minor processing errors, such as file access problems, are not reported.

Output is normally to the standard output but may be directed to a file named by the "-o"
OUTPUT option. As with logging, the special names "[stdout]" and "[stderr]" are also
recognized and cause the survey report to be sent to the indicated stream.

A few of the report topics are very verbose, such as the list of variable names. These
categories are generally reported as a truncated list with the last element summarizing
the unseen remainder ("plus 45000 more"). The length of such output lists is controlled by
tht LINES option, "-lines." The special value 0 means all lines. (Beware that "all" can
imply a very large list: the Go corpus has 480,556 unique identifiers.)

Surveying uses multiple CPUs to maximize performance. How many is controlled by the CPU
option "-cpu" whose default value of 0 means "all."" Force a single process with "-cpu=1"
and likewise to force any desired level of concurrency. The report will show CPU scaling
efficiency when multiple CPUs are in use. To understand it, consider the system's number
and nature of processors. Machines may have N cores and N virtual cores, reported as 2N
CPUs, but with a throughput of 1.25 to 1.5 N. Specifically, a 4-core + 4 SMT core intel
processor has 5 to 6 cores of performance. If the reported speedup there is 5x to 6x,
that is efficient. Values of 1.6 to 1.7 times the non-SMT core count seem all that may be 
expected so rejoice in Go's efficiency despite asymmetric SMT.

Author: Michael Jones
`

func main() {
	// parse command line before configuring logging (to allow "-log xyz.txt")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s", usage)
	}
	flag.Parse()

	// set logging format and destination before first log event
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	switch strings.ToLower(*flagLog) {
	case "":
		// no logging
	case "[stdout]":
		log.SetOutput(os.Stdout)
	case "[stderr]":
		log.SetOutput(os.Stderr)
	default:
		file, err := os.Create(*flagLog)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(file)
	}

	// control concurrency for testing (no disadvantage for maximal concurrrency)
	if *flagCPUs != 1 {
		if *flagCPUs == 0 {
			// claim CPUs
			*flagCPUs = runtime.NumCPU()
		} else if *flagCPUs < 0 {
			// spare CPUs
			*flagCPUs += runtime.NumCPU() // "-cpu -2" ==> "max(num CPUs - 2, 1)"
			if *flagCPUs < 1 {
				*flagCPUs = 1
			}
		}
	}

	// nornalize style options to lower case, expand nicknames
	*flagStyle = strings.ToLower(*flagStyle)
	if *flagStyle == "md" {
		*flagStyle = "markdown"
	}

	// perform actual work
	switch {
	case *flagSplit != "":
		//split a large Go-code "blob" (such as the Go-Corpus) into parts.
		doSplit()
	default:
		// survey Go-code in files, directories, hierchies, archives, and blobs.
		doSurvey()
	}
}
