// Package atmos is the air over a map: the climate of the wind, worked out
// from the warmth of the air and the lie of the ground; the water it carries
// off the sea and rains out, and the rain the ground wrings from it; the sea's
// currents; the energy balance that gives each latitude its year; and the
// day's weather moving through that climate.
//
// It knows a map only as its size and the ground as the air reads it: how far
// each tile stands over the water the air takes its fill from, and which tiles
// are under it. The land reads the rain and the wind back off it tile by tile;
// see weather.go and sky.go in the root package.
package atmos
