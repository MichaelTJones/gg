#!/bin/sh

go build -o gg main.go scan.go

options="a aC aD aI aK aN aO aP aR aS aV"
queries="grep for test 2 true -42 5.25 -5.25"

err=false
for o in $options; do
    for q in $queries; do
        ./gg -cpu=1 -summary=false -log=a $o $q . > ./new
        gg -cpu=1 -summary=false -log=b $o $q . > ./old
        CHANGES=$(diff ./new ./old | wc -l)

        if [ $CHANGES -eq 0 ]; then
            rm -rf ./new ./old ./a ./b
        else
            echo "Outputs don't match for 'gg $o $q .'"
            diff ./new ./old
            err=true
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
rm -rf ./gg
