/*
Package txtarpkg provides utilities for creating and extracting txtar archives.

Txtar is a simple text-based archive format used by the Go project for
bundling multiple text files into a single file. This package offers
convenient functions for working with txtar files programmatically.

# Features

The package provides:
  - Archive creation from multiple files
  - Archive extraction to disk
  - In-memory archive manipulation
  - Format validation

# Example

Create a new archive:

	archive := &txtar.Archive{
		Files: []txtar.File{
			{Name: "hello.txt", Data: []byte("Hello, World!")},
			{Name: "readme.txt", Data: []byte("This is a readme.")},
		},
	}
	data := txtar.Format(archive)

Extract an archive:

	archive := txtar.Parse(data)
	for _, file := range archive.Files {
		fmt.Printf("%s: %s\n", file.Name, file.Data)
	}
*/
package txtarpkg