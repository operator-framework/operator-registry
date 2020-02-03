package db

type pager interface {
	// load a page from storage.
	page(n int, pagesize int) ([]byte, error)
	// as it says
	Close() error
	// read lock
	RLock() error
	// unlock read lock
	RUnlock() error
	// true if there is any 'RESERVED' lock on this file
	CheckReservedLock() (bool, error)
}
