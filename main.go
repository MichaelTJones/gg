/*
   gg is classic grep (g/RE/p) with Go knowledge to search package names,
   numbers, identifiers, comments, keywords, and other language tokens.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

// common flags
var flagCPUs = flag.Int("cpu", -1, "number of CPUs to use (0 for all)")
var flagGo = flag.Bool("go", true, `limit grep to Go files ("main.go")`)
var flagList = flag.String("list", "", "list of filenames to grep")
var flagLog = flag.String("log", "", `write log to named file (or "[stdout]" or "[stderr]")`)
var flagOutput = flag.String("output", "", `write output to named file (or "[stdout]" or "[stderr]")`)
var flagRecursive = flag.Bool("r", false, "grep directories recursively")
var flagVisible = flag.Bool("visible", true, `limit grep to visible files (skip ".hidden.go")`)

// grep-compatibility flags
var flagActLikeGrep = flag.Bool("g", false, "act like grep")
var flagFileName = flag.Bool("h", false, `disply file name ("header") for each match`)
var flagLineNumber = flag.Bool("n", false, "disply line number for each match")

// secret developer flags
var flagSummary = flag.Bool("summary", false, "print performance summary")
var flagBufferWrites = flag.Bool("bufferWrites", true, "buffer output writes")
var flagBufferSize = flag.Int("bufferSize", 64*1024, "output buffer size")
var flagTrim = flag.Bool("trim", false, "trim matched strings")
var flagProfileCPU = flag.String("cpuprofile", "", "write cpu profile to file")
var flagProfileMem = flag.String("memprofile", "", "write memory profile to file")
var flagUnordered = flag.Bool("unordered", false, "disregard file traversal order")

// usage string is the whole man page
var usage = `NAME
    gg - grep Go-language source code

SYNOPSIS
    gg [options] acdiknoprstvg regexp [file ...]

DESCRIPTION
    gg is classic grep (g/RE/p) with flag-directed Go token focus to search
    in package names, numbers, identifiers, comments, keywords, and more.
    Token flags are "acdiknoprstvg" in any order or combination:

       a   search in All of the following
       c   search in Comments (//... or /*...*/)
       d   search in Defined non-types (iota, nil, new, true,...)
       i   search in Identifiers ([alphabetic][alphabetic | numeric]*)
       k   search in Keywords (if, for, func, go, ...)
       n   search in Numbers ("255" matches 255, 0.255, 1e255)
       o   search in Operators (,  +  -  *  /  [  ] {  }  ( )  >>...)
       p   search in Package names
       r   search in Rune literals ('a', '\U00101234')
       s   search in Strings (quoted or raw)
       t   search in Types (bool, int, float64, map, ...)
       v   search in Values (255 is 0b11111111, 0377, 255, 0xff)
       g   search as grep, perform simple line-by-line matches in file

    gg combines lexical analysis and Go-native pattern matching to extend
    grep(1) for Go developers.  The search is restricted, seeking matches
    only in chosen token classes.  A search in number literals can match
    values, "v 255" matches the numeric value 255 in source code as
    0b1111_1111, 0377, 0o377, 255, 0xff, etc.  Go's linear-time regular
    expression engine is Unicode-aware and supports many Perl extensions:
    numbers in identifiers are found with "gg i [0-9]" or "gg i [\d]",
    comments with math symbols by "gg c \p{Sm}", and Greek in strings via
    "gg s \p{Greek}" each with appropriate shell escaping.

    gg searches files names listed on the command line or in a file of
    filenames provided the "-list" argument.  If neither of these is
    present, gg reads file names from the standard input which is useful in
    shell pipelines such as "find . -name "*.go" | gg k fallthrough"

    Files are Go source code files or directories.  Source files include
    typical ".go" files; compressed ".go" files named ".go.bz2", ".go.gz",
    or ".go.zst" for Bzip2, Gzip, and ZStandard compression formats;
    archives of any such files in the formats "a.cpio", "a.tar", or
    "a.zip"; or, finally, compressed archives as in "a.cpio.bz2" and
    "a.tar.gz".  If a named file is a directory then all Go source files
    in that directory are scanned without visiting subdirectories.  With
    the "-r" flag enabled, named directories are processed recursively,
    scanning each Go source file or archive in that directory's hierarchy.

OPTIONS
    -cpu=n
        Set the number of CPUs to use. Negative n means "all but n."
        Default is all.

    -go=bool
        Limit search to ".go" files.  Default is true.

    -h=bool
        Display file names ("headers") on matches.  Default is false for
        single-file searches and true otherwise.

    -list=file
        Search files listed one per line in the named file.

    -log=file
        Write a log of execution details to a named file.  The special
        file names "[stdout]" and "[stderr]" refer to the stdout and
        stderr streams.  (Last line of log details efficiency.)

    -n=bool
        Display line numbers following each match. Numbers count from
        one per file.  Default is false.

    -output=file
        gg output is normally to stdout but may be directed to a named
        file.  The special names "[stdout]" and "[stderr]" refer to the
        stdout and stderr streams.

    -r=bool
        Search directories recursively.  Default is false.

    -visible=bool
        Restrict search to visible files, those with names that do not
        start with "." (in the shell tradition).  Default is true.

    acdiknoprstvCDIKNOPRSTVg
        The Go token class flags have an upper case negative form to
        disable the indicated class.  Used with "a" for "all", "aCS"
        means "search All tokens except Comments and Strings."  Flag "g"
        means search as if the grep command, ignore Go lexical analysis
        and match lines.

EXAMPLES
    To search for comments containing "case" (ignoring switch statements)
    in every ".go" file in the current working directory, use the command:

        gg c case .

    To find number literals containing the digits 42 in ".go" files located
    anywhere in the current directory's hierarchy, use the command:

        gg -r n 42 .

    Find numbers with values of 255 (0b1111_1111, 0377, 0o377, 255, 0xff)
    in ".go" files in the gzipped tar(1) archive omega with the command:

        gg v 255 omega.tar.gz

AUTHOR
    Michael T. Jones (https://github.com/MichaelTJones)

SEE ALSO
    https://golang.org/pkg/regexp/syntax/
    https://github.com/google/re2/wiki/Syntax
    https://en.wikipedia.org/wiki/Unicode_character_property
`

func main() {
	// parse command line to allow access to profiling options in doProfile()
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s", usage)
	}
	flag.Parse()

	// launch program
	programStatus := doProfile()

	// return program status to shell
	os.Exit(programStatus)
}

// profile the program's execution
func doProfile() int {
	if *flagProfileCPU != "" {
		f, err := os.Create(*flagProfileCPU)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not create CPU profile: %v\n", err)
			return 2 // grep-compatible code for program error
		}
		defer func() {
			f.Close()
			fmt.Fprintf(os.Stderr, "cpu profile recorded in %s\n", *flagProfileCPU)
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "could not start CPU profile: %v\n", err)
			return 2 // grep-compatible code for program error
		}
		defer pprof.StopCPUProfile()
	}

	// execute the program
	programStatus := doMain()

	if *flagProfileMem != "" {
		f, err := os.Create(*flagProfileMem)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not create memory profile: %v\n", err)
			return 2 // grep-compatible code for program error
		}
		defer f.Close()
		defer func() {
			f.Close()
			fmt.Fprintf(os.Stderr, "memory profile recorded in %s\n", *flagProfileMem)
		}()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "could not write memory profile: %v\n", err)
			return 2 // grep-compatible code for program error
		}
	}

	// trigger completion of profiling and return status
	return programStatus
}

func doMain() int {
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
			log.Print(err)
			return 2
		}
		log.SetOutput(file)
	}

	// control concurrency (for testing and tuning)
	*flagCPUs = getMaxCPU()

	// bonus feature:
	// If you make a symbolic link to the executable or otherwise rename it from "gg" then it
	// will automatically run in "be like grep" mode without needing the "g" or any other flag.
	// if !strings.HasSuffix(os.Args[0], "gg") {
	// 	*flagActLikeGrep = true // if user's made a symlink or renamed, become grep
	// }

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: gg [flags] acdiknoprstvg regexp [file ...]\n")
		fmt.Fprintf(os.Stderr, "    gg -help for details\n")
		return 2 // failure: (like grep: return 2 instead of 1)
	}

	if *flagRecursive {
		*flagFileName = true
	}

	// perform actual work
	start := time.Now()
	s, err := doScan()
	elapsed := time.Since(start).Seconds()
	user, system, _ := getResourceUsage()

	// print performance summary
	if *flagLog != "" {
		s.print(elapsed, user, system, printf) // print to log
	}
	if *flagSummary {
		s.print(elapsed, user, system, func(f string, v ...interface{}) {
			_, _ = fmt.Printf(f, v...) // print to stdout
		})
	}

	// return grep-compatible program status
	programStatus := 0
	switch {
	case err != nil:
		printf("error: %v", err)
		programStatus = 2 // program failure: (like grep)
	case s.matches <= 0:
		programStatus = 1 // search unsuccessful: no match; handy in shell "&&" constructs
	default: // err==nil && s.matches>=1
		programStatus = 0 // search successful: 1 or more matches
	}
	return programStatus
}

func getMaxCPU() int {
	// honor cpu option flag...
	cpus := runtime.NumCPU() // default is all CPUs
	switch {
	case *flagCPUs > 0:
		cpus = *flagCPUs // claim N CPUs (+2 means "use 2 CPUs")
	case *flagCPUs < 0:
		cpus = *flagCPUs + cpus // spare N CPUs (-2 means "use all but 2 CPUs")
	}
	// ...but allow at least 2 scan worker goroutines
	if cpus < 2 {
		cpus = 2
	}
	return cpus
}
