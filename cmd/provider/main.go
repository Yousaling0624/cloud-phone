// Command cloud-phone-provider runs the demo provider gateway on a fixed
// address, so a standalone server can be pointed at it through
// provider_base_url instead of the in-process demo.
package main

import (
	"flag"
	"log"

	"cloud-phone/internal/provider/demo"
)

func main() {
	var (
		listen   = flag.String("listen", "127.0.0.1:8790", "HTTP listen address")
		token    = flag.String("token", "demo", "bearer token clients must present")
		failRate = flag.Float64("fail-rate", 0, "login failure rate, 0..1")
	)
	flag.Parse()

	p, err := demo.Start(demo.Options{Listen: *listen, Token: *token, FailRate: *failRate})
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()
	log.Printf("cloud-phone-provider listening on %s", p.URL)
	// The provider is a developer tool; the token is how the operator
	// configures the platform, so print it once at startup.
	log.Printf("provider token: %s", p.Token)
	select {}
}
