Go Type Replacer
======

Problem: "I want to replace my Foo type with external.Foo through my entire codebase (and fix imports too!)"

Solution 1: Use find/replace/manul import fixups
Solution 2: Use this app!

Quick Guide
--

`gotypereplace --import github.com/foo/bar --replace myfoo.Foo:yourfoo.AnotherFoo --inplace --dir .`

`--import` is used to provide the import text for files which are being modified (if they don't already import it)
`--replace` is a tuple of `from`:`to` where from is the package.ident name to replace and to is the package.ident to replace it with.

Multiple `--replace` items can be provided, each will be run on each file processed.

`--inplace` will write the resulting code to the input file, if this is not present it will write to stdout
`--dir` provides the directory to walk. If this is not provided one or more filenames can be provided on the command line instead.

Hope this is useful!

Jonathan