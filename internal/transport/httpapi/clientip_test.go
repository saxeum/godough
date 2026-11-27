package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The header is attacker-controlled on every hop except the last one, which is the
// only part of this worth being precise about: a client that sends its own
// X-Forwarded-For has its value kept at the front and the ALB's address appended at
// the back, so reading the last entry is what makes it impossible to choose your own
// session address by asking.
func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		want       string
	}{
		{
			name:       "no header falls back to the peer",
			remoteAddr: "10.42.10.1:51234",
			want:       "10.42.10.1",
		},
		{
			name:       "one hop is the client",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "a spoofed left entry is ignored and the appended one wins",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"1.2.3.4, 203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "several headers are one header",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"1.2.3.4", "203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "whitespace and empty elements",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"  , 203.0.113.7 ,  "},
			want:       "203.0.113.7",
		},
		{
			name:       "a recorded port is dropped",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"203.0.113.7:4711"},
			want:       "203.0.113.7",
		},
		{
			name:       "ipv6 keeps its canonical form",
			remoteAddr: "[2001:db8::1]:51234",
			forwarded:  []string{"2001:0db8:0000:0000:0000:0000:0000:0007"},
			want:       "2001:db8::7",
		},
		{
			name:       "bracketed ipv6 with a port",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"[2001:db8::7]:443"},
			want:       "2001:db8::7",
		},
		{
			name:       "garbage is not an address, so the peer is the answer",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"not-an-address"},
			want:       "10.42.10.1",
		},
		{
			name:       "an injected value cannot smuggle a second address past the last",
			remoteAddr: "10.42.10.1:51234",
			forwarded:  []string{"203.0.113.7, <script>"},
			want:       "10.42.10.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/snippet", nil)
			r.RemoteAddr = tt.remoteAddr
			for _, v := range tt.forwarded {
				r.Header.Add("X-Forwarded-For", v)
			}

			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
