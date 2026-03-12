Place your application files here.

This directory is embedded into the installer binary via go:embed.
All files in this directory will be extracted to the install directory
(C:\Program Files\<AppName>\) during installation.

In this example we have example.go in this directory so you can `go build`
to get a sample program already in the right place. Normally you wouldn't
include source files in this directory. Only release artifacts such as EXE
files or assets.
