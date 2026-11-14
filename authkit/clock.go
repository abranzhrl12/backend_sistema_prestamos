package authkit

import "time"

// Clock abstrae el tiempo para poder inyectarlo (y fijarlo en tests).
type Clock interface{ Now() time.Time }

// SystemClock usa el reloj real.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
