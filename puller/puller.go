package puller

// Puller an interface for pull
type Puller interface {
	Pull([]string) error
}
