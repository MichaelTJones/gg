#!/bin/sh

go build -o gg *.go

options="a aC aD aI aK aN aO aP aR aS aV"
queries="grep for test 2 true -42 5.25 -5.25"
sources=". testdata/*"

err=false
for o in $options; do
    for q in $queries; do
        for s in $sources; do
            ./gg -cpu=1 -summary=false -log=a $o $q $s > ./new
            gg -cpu=1 -summary=false -log=b $o $q $s > ./old
            CHANGES=$(diff ./new ./old | wc -l)

            if [ $CHANGES -eq 0 ]; then
                rm -rf ./new ./old ./a ./b
            else
                echo "Outputs don't match for 'gg $o $q $s'"
                diff ./new ./old
                err=true
                break
            fi
            if [ $err = true ]; then
                break
            fi
        done
        if [ $err = true ]; then
            break
        fi
    done
    if [ $err = true ]; then
        break
    fi
done

if [ $err = false ]; then
    echo "Nice, everything is still working!"
fi
rm -rf ./gg ./a ./b
