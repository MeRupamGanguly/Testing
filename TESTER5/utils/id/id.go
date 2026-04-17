package id

/*
 * IDs generated within the same millisecond still have a deterministic, increasing order
 * (e.g., 1st ID < 2nd ID < 3rd ID). Without it, two IDs created at the exact same time could have random ordering when sorted lexicographically.
 * ULID’s monotonic entropy guarantees total order even under high concurrency. Ideal for distributed systems like e‑commerce backends.
 */
import (
	"time"

	"github.com/oklog/ulid/v2"
)

// Generator defines the interface for ID generation.
type Generator interface {
	New() string
	Time(id string) (time.Time, error)
}

// ULIDGenerator implements ULID (sortable by time).
type ULIDGenerator struct {
	entropy *ulid.MonotonicEntropy
}

func NewULIDGenerator() *ULIDGenerator {
	entropy := ulid.Monotonic(ulid.DefaultEntropy(), 0)
	return &ULIDGenerator{entropy: entropy}
}

func (g *ULIDGenerator) New() string {
	t := time.Now().UTC()
	uid := ulid.MustNew(ulid.Timestamp(t), g.entropy)
	return uid.String()
}

func (g *ULIDGenerator) Time(idStr string) (time.Time, error) {
	uid, err := ulid.Parse(idStr)
	if err != nil {
		return time.Time{}, err
	}
	// Convert from milliseconds to time.Time
	t := time.Unix(int64(uid.Time())/1000, (int64(uid.Time())%1000)*1000000).UTC()
	return t, nil
}

// DefaultGenerator is the ULID generator.
var DefaultGenerator Generator = NewULIDGenerator()

// NewID generates a new ID using the default generator.
func NewID() string {
	return DefaultGenerator.New()
}

// ParseTime extracts the timestamp from an ID.
func ParseTime(idStr string) (time.Time, error) {
	return DefaultGenerator.Time(idStr)
}
