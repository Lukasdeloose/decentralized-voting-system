package helpers

///*** main constants ***///

const MaxBufferSize = 8192 * 2

///*** clients constants ***///

const PRIVATE = 1
const GOSSIP = 2
const FILE = 3
const REQUEST = 4
const SEARCH = 5

///*** file sharing constants ***///

const ChunkSize = 8192 //8KiB
const HashSize = 32    //256 bit
const MaxBudget = 32
const MatchThreshold = 2
const SearchChannelSize = 25
const SearchTimeout = 1
const DuplicateSearchTimeout = 0.5

///*** Gossiper constants ***///

const Localhost = "127.0.0.1"
var HopLimit uint32

///*** Blockchain constants ***///
var StubbornTimeout = 5
var Nodes = 0
const TLCBufferSize = 20


///*** TLC constants ***///
const OLDROUND = 0
const SAMEROUND = 1
const FUTUREROUND = 2
