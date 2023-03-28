module github.com/terwey/recordwriter

go 1.17

require github.com/productsupcom/go-sqlite-lite v0.9.5

require (
	github.com/bvinc/go-sqlite-lite v0.6.1 // indirect
	github.com/google/go-cmp v0.5.9
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/terwey/csv2sqlite v0.23.0-rc // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/term v0.6.0
)

// the upstream seems unmaintained
replace github.com/bvinc/go-sqlite-lite v0.6.1 => github.com/productsupcom/go-sqlite-lite v0.6.1
