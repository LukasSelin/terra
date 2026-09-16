// cmd/zarr is a module of its own so that terra's go.mod stays the standard
// library's alone: this is the one place the world and the zarr module meet.
module github.com/LukasSelin/terra/cmd/zarr

go 1.27.0

require (
	github.com/LukasSelin/terra v0.0.0
	github.com/LukasSelin/zarr v0.1.0
)

replace github.com/LukasSelin/terra => ../..
