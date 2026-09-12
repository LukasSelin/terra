// terra is a world: the land, the weather over it, what grows on it and the
// way across it. It is its own module so that more than one game can be
// built on one world - which is what it is for.
//
// It depends on nothing but the standard library. That is not an accident
// and it is worth keeping: a country does not need a terminal, a simulation
// of people, or a vocabulary for what anybody wants to do with it.
//
// It is expected to sit beside whatever is built on it, and is reached by a
// replace directive until it is published:
//
//	repos/
//	  terra/   <- here
//	  lreat/
module github.com/LukasSelin/terra

go 1.27.0
