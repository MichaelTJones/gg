package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

var flagCPUs = flag.Int("cpu", 0, "number of CPUs to use (0 for all)")
var flagGo = flag.Bool("go", true, `limit grep to Go files ("main.go")`)
var flagList = flag.String("list", "", "list of filenames to grep")
var flagLog = flag.String("log", "", `write log to named file (or "[stdout]" or "[stderr]")`)
var flagOutput = flag.String("output", "", `write output to named file (or "[stdout]" or "[stderr]")`)
var flagRecursive = flag.Bool("r", false, "grep directories recursively")
var flagVisible = flag.Bool("visible", true, `limit grep to visible files (skip ".hidden.go")`)

// grep-compatibility flags
var flagFileName = flag.Bool("h", false, "disply file name for each match")
var flagLineNumber = flag.Bool("n", false, "disply line number for each match")

var usage = `NAME
     gg − grep Go‐language source code

SYNOPSIS
    gg [options] agcdiknoprstv regexp [file ...]

DESCRIPTION
    gg is classic grep (g/RE/p) with Go token search direction: identifiers,
    package names, numbers, comments, keywords, and the like. The flags and
    classes include "agcdiknoprstv" in any order and combination.

        a: search in All of the following
        c: search in Comments (//... or /*...*/)
        d: search in Defined non‐types (iota, nil, new, true,...)
        i: search in Identifiers ([alphabetic][alphabetic | numeric]*)
        k: search in Keywords (if, for, func, go, ...)
        n: search in Numbers ("255" matches 255, 0.255, 1e255)
        o: search in Operators ( ,  +  ‐  *  /  [  ] {  }  ( )  >> ...)
        p: search in Package names
        r: search in Rune literals (’a’, ’\U00101234’)
        s: search in Strings (quoted or raw)
        t: search in Types (bool, int, float64, map, ...)
        v: search in Values (255 is 0b11111111, 0377, 255, 0xff)
        g: search as grep, perform line-by-line matches in file

    gg  combines  lexical analysis and Go‐native pattern matching to extend
    grep(1) for Go developers.  The search is restricted,  seeking  matches
    only  in  chosen  token classes.  A search in number literals can match
    values, "v 255" matches  the  numeric  value  255  in  source  code  as
    0b1111_1111,  0377,  0o377,  255,  0xff, etc.  Go’s linear‐time regular
    expression engine is Unicode-aware and supports many  Perl  extensions,
    so  numbers in identifiers are found by "gg i [0‐9]" or "gg i [\d]" and
    comments containing math symbols are found  by  "gg  c  \p{Sm}"   (with
    appropriate shell escaping). Refer to SEE ALSO for details.

    gg  searches  files named on the command line or in a list of filenames
    provided the "‐list" argument.  If neither is present,  gg  reads  file
    names  from the standard input.  This last is useful in shell pipelines
    such as "find . ‐name "*.go" | gg k fallthrough"

    Files are Go source files or directories.  Source files include typical
    ".go"  files;  compressed  ".go"  files  named  ".go.bz2", ".go.gz", or
    ".go.zst" for Bzip2, Gzip, and ZStandard compression formats;  archives
    of  any  such  files  in the formats "a.cpio", "a.tar", or "a.zip"; or,
    finally, compressed archives as in "a.cpio.bz2" and "a.tar.gz".   If  a
    named  file  is  a directory then all Go source files in that directory
    are scanned  without  visiting  subdirectories.   With  the  "‐r"  flag
    enabled,  named directories are processed recursively, scanning each Go
    source file in that directory’s hierarchy.

OPTIONS
    −cpu=n Set the number of CPUs to use. Negative n  means  "all  but  n."
        Default is all.

    −go=bool
        Limit search to ".go" files.  Default is true.

    −h=bool
        Display file names ("headers") on matches.  Default is false for
        single‐file searches and true otherwise.

    −list=file
        Search files listed one per line in the named file.

    −log=file
        Write a log of execution details to a named file.   The  special
        file  names  "[stdout]"  and  "[stderr]" refer to the stdout and
        stderr streams.

     −n=bool
        Display line numbers with matches.  Default is false.

    −output=file
        gg output is normally to stdout but may be directed to  a  named
        file.   The special names "[stdout]" and "[stderr]" refer to the
        stdout and stderr streams.

	−r=bool
        Search directories recursively.  Default is false.

	−visible=bool
        Restrict search to visible files, those with names that  do  not
        start with "." (in the shell tradition).  Default is true.

    agcdiknoprstvCDIKNOPRSTV
        The  Go  token  class  flags have an upper case negative form to
        disable the indicated class.  Used with  "a"  for  "all",  "aCS"
        means "search All tokens except Comments and Strings."  Flag "g"
        means search as if the grep command, ignore Go lexical  analysis
        and match lines.

EXAMPLES
    To  search  for comments containing "case" (ignoring switch statements)
    in the ".go" files of the current working directory, use the command:

        gg c case .

    To find number literals containing the digits 42 in ".go" files located
    anywhere in the current directory’s hierarchy, use the command:

        gg ‐r n 42 .

    Find  numbers  with values of 255 (0b1111_1111, 0377, 0o377, 255, 0xff)
    in ".go" files in the gzipped tar(1) archive omega with the command:

        gg v 255 omega.tar.gz

AUTHOR
    Michael T. Jones (https://github.com/MichaelTJones)

SEE ALSO
    https://golang.org/pkg/regexp/syntax/
    https://en.wikipedia.org/wiki/Unicode_character_property
`

func main() {
	// parse command line before configuring logging (to allow "-log xyz.txt")
	flag.Usage = func() {
		// fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		// flag.PrintDefaults()
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
		if *flagCPUs == 0 { // claim CPUs
			*flagCPUs = runtime.NumCPU()
		} else if *flagCPUs < 0 { // spare CPUs
			*flagCPUs += runtime.NumCPU() // "-cpu -2" ==> "all but 2 CPUs"
			if *flagCPUs < 1 {
				*flagCPUs = 1
			}
		}
	}

	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "usage: gg [flags] tokens regexp [file ...]\n")
		os.Exit(1)
	}

	if *flagRecursive {
		*flagFileName = true
	}

	// perform actual work
	doScan()
}
