// cmd/zarr is a module of its own so that terra's go.mod stays the standard
// library's alone: this is the one place the world and the zarr module meet.
module github.com/LukasSelin/terra/cmd/zarr

go 1.27.0

require (
	github.com/LukasSelin/terra v0.0.0
	github.com/LukasSelin/zarr v0.3.0
	github.com/LukasSelin/zarr/zstd v0.0.0-20260918083450-a273522960fa
)

require github.com/klauspost/compress v1.20.0 // indirect

replace github.com/LukasSelin/terra => ../..
