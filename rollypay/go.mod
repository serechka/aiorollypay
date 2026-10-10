// Stub module that retracts the abandoned module path
// github.com/serechka/aiorollypay/rollypay. The module moved to the
// repository root: github.com/serechka/aiorollypay.
//
// Retraction persists through the Go module proxy even if this directory
// is later removed from HEAD.
module github.com/serechka/aiorollypay/rollypay

go 1.22

retract [v0.0.0, v0.1.0]
