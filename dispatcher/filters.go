package dispatcher

// Filter interface
type Filter interface {
	PassThrough(Update) bool
}
