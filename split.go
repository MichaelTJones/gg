package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/MichaelTJones/lex"
)

/*
Split named Go "blob" into individual files by (a) taking note of the Go-language package
statement that must appear near the beginning of each ".go" file, and (b) by using the lex
Go-lexer to track commemts, whitespace, and package statements in the blob. This allows
the split logic to "back up" before the package statement to the start of comments and
whitespace. While there is no absolute way to know which were at the end of the prior file
and which were at the start of the present, experience shows it to be all header and no
footer; in nearly every case the comments are indeed package start comments or else a
copyright/disclaimer notice, each of which pertain to the present file. doSplit() manages
this carefully and splits the "blob" back into the constituent parts. (Though, the file
names are lost.)

For performance, the Go archive's "Giant Blob" style is too monolithic. It is sub-optimal
because portions are not easily read or surveyed concurrently. The myriad individual files
style would not be great either, as it implies far too much filesystem interaction and
"ls" in a 60k file directory is a bad experience. Best is a medium number of groups, say
2x to 10x the number of CPUs, each of which represents the concatenation of dozens or
hundreds of individual Go source code files. The grouping logic in doSplit() uses the SIZE
option ("-size") to control grouping. As soon as the size of accumulated input files
exceeds this minimum size, the group is written into the working directory indicated by
flagOutput (or the current working directory if unset).

Example, split the Go corpus (752 MB) into files of ~4MB:

    survey -split corpus -size 4000000 -o fragments

produces 176 fragments each just over 4MB. Use them directly for fastest surveying
("survey fragments/*.go"), compessed ("zstd -19 fragments/*.go") and surveyed in that form
("survey fragments/*.zst"), or archived. Archiving compressed versions (".go.zst") with
tar ("(cd fragments; tar cf ../corpus.tar corpus*.zst)") yields a smaller 92,386,304 byte
corpus.tar, whose 176 parts are automatically extracted from the archive and decompressed
in parallel as part of surveying ("survey corpus.tar"):

    176 files (source code groups)
    62780 files (original source code files)
    22927078 lines (14176491 per sec)
    116926048 tokens (72298838 per sec)
    752311514 bytes (465176484 per sec)
    1.617260 seconds to read/lex/tabulate/sort/adjust
    36 workers (parallel speedup = 26.02 x with SMT)

The log file:

    2019/06/25 08:40:50.701984 survey begins
    2019/06/25 08:40:50.702123 processing files listed on command line
    2019/06/25 08:40:50.702162 processing tar archive corpus.tar
    2019/06/25 08:40:50.739386     207580 →  4065186 bytes (19.584×)  decompress and survey corpus.tar::corpus_000009.go.zst
    2019/06/25 08:40:50.742892     588144 →  4058916 bytes ( 6.901×)  decompress and survey corpus.tar::corpus_000000.go.zst
    2019/06/25 08:40:50.760732      93940 →  4666515 bytes (49.675×)  decompress and survey corpus.tar::corpus_000004.go.zst
    2019/06/25 08:40:50.760864     206993 →  4023123 bytes (19.436×)  decompress and survey corpus.tar::corpus_000005.go.zst
    2019/06/25 08:40:50.760978     252664 →  4343752 bytes (17.192×)  decompress and survey corpus.tar::corpus_000001.go.zst
                :
    2019/06/25 08:40:51.855720     731744 →  4013175 bytes ( 5.484×)  decompress and survey corpus.tar::corpus_000171.go.zst
    2019/06/25 08:40:51.859625     717740 →  4319130 bytes ( 6.018×)  decompress and survey corpus.tar::corpus_000170.go.zst
    2019/06/25 08:40:51.860909     534458 →  2880005 bytes ( 5.389×)  decompress and survey corpus.tar::corpus_000175.go.zst
    2019/06/25 08:40:51.861120     619070 →  4002126 bytes ( 6.465×)  decompress and survey corpus.tar::corpus_000174.go.zst
    2019/06/25 08:40:51.876724     518866 →  4350379 bytes ( 8.384×)  decompress and survey corpus.tar::corpus_000173.go.zst
    2019/06/25 08:40:52.325722 filetypes processed: ".go", ".tar", ".zst"
    2019/06/25 08:40:52.325798 processed 176 good and 0 bad files in 1.622749032 seconds
    2019/06/25 08:40:52.325804 survey ends
    2019/06/25 08:40:52.325808 report begins
    2019/06/25 08:40:52.608179 report ends

With size set to zero. 62,780 groups would have been written, one per file. (Note: the
curiously large compression ratios such as 50:1 above correspond to protobuf generated
source code where regularities are captured by Zstandard at high ("-19") effort levels.)
*/

func doSplit() {
	println("split begins")

	subdir := "parts"
	if *flagOutput != "" {
		subdir = *flagOutput
	}
	println("  group destination: ", subdir)
	println("  group byte target: ", *flagSize)

	// read file
	filename := *flagSplit
	filebase := filepath.Base(filename)
	filehead := strings.TrimSuffix(filebase, filepath.Ext(filebase))
	file, err := os.Open(filename)
	if err != nil {
		println(err)
		return
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file) // the Go Corpus is 752 MB
	if err != nil {
		println(err)
		return
	}
	source := string(bytes)
	printf("  %d byte%s, file %q", len(source), plural(len(source), ""), filename)

	var group []string
	var groupBytes, groupCount, totalBytes, packageCount, bodyStart int
	suffixStart := -1

	lexer := &lex.Lexer{Input: source, Mode: lex.ScanGo} // skip no Go token, not even whitespace
	for tok, text := lexer.Scan(); tok != lex.EOF; tok, text = lexer.Scan() {
		if tok == lex.Keyword && text == "package" {
			if packageCount == 0 { // first package of file: keep accumulating until next one
				suffixStart = -1 // this is not a package divider, so discard this run of comments and whitespace
				packageCount++
			} else {
				if suffixStart == -1 {
					suffixStart = lexer.Offset // no package comment
				} else if source[suffixStart] == '\n' {
					suffixStart++ // associate initial '\n' in whitespace with last line of body
				}
				body := source[bodyStart:suffixStart]
				group = append(group, body)
				groupBytes += len(body)
				if groupBytes >= *flagSize {
					totalBytes += groupBytes
					fragment := fmt.Sprintf("%s_%06d.go", filehead, groupCount)
					writeFile(subdir, fragment, group)
					printf("  fragment %q  %5d package%s   %9d byte%s\n",
						fragment,
						len(group), plural(len(group), " "),
						groupBytes, plural(groupBytes, " "))
					groupCount++
					group = group[:0]
					groupBytes = 0
				}
				bodyStart = suffixStart // associate these comments with the next package statement
				suffixStart = -1
				packageCount++
			}
		} else if tok == lex.Comment || tok == lex.Space {
			if suffixStart == -1 {
				suffixStart = lexer.Offset
			}
		} else {
			suffixStart = -1 // not a package statement, so discard prefix comments and whitespace
		}
	}
	// output final part
	body := source[bodyStart:]
	group = append(group, body)
	groupBytes += len(body)
	totalBytes += groupBytes
	fragment := fmt.Sprintf("%s_%06d.go", filehead, groupCount)
	writeFile(subdir, fragment, group)
	printf("  fragment %q  %5d package%s   %9d byte%s\n",
		fragment,
		len(group), plural(len(group), " "),
		groupBytes, plural(groupBytes, " "))
	groupCount++

	printf("  %d byte%s, %d group%s, %d package%s",
		totalBytes, plural(totalBytes, ""),
		groupCount, plural(groupCount, ""),
		packageCount, plural(packageCount, ""))
	println("split ends")
}

func writeFile(subdir, name string, parts []string) error {
	var file *os.File
	switch lower := strings.ToLower(name); {
	case lower == "[stdout]":
		file = os.Stdout
	case lower == "[stderr]":
		file = os.Stderr
	case lower != "":
		var err error
		if subdir != "" {
			err = os.MkdirAll(subdir, os.ModePerm)
			if err != nil && err != os.ErrExist {
				println(err)
				return err
			}
		}
		file, err = os.Create(filepath.Join(subdir, name))
		if err != nil {
			println(err)
			return err
		}
		defer file.Close()
	}
	for _, s := range parts {
		file.WriteString(s)
	}
	return nil
}
