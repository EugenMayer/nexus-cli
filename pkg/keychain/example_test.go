package keychain_test

import (
	"context"
	"fmt"

	"github.com/EugenMayer/nexus-cli/pkg/keychain"
)

func Example() {
	defer keychain.MockInit()() // in-memory keyring for this example; omit in real code

	kc := keychain.New("mytool")
	_ = kc.Set("jira.token", "s3cret")

	token, _ := kc.Get("jira.token")
	service, account := kc.Entry("jira.token")
	fmt.Println(token, "stored as", service, account)
	// Output: s3cret stored as mytool.jira.token mytool
}

func ExampleResolver() {
	defer keychain.MockInit()()
	kc := keychain.New("mytool")
	_ = kc.Set("gitlab.token", "from-keyring")

	r := &keychain.Resolver{
		Keychain: kc,
		// fallback for configs that still hold the value in plain text
		Plain: func(path string) string { return map[string]string{"jira.token": "from-config"}[path] },
	}

	for _, path := range []string{"gitlab.token", "jira.token"} {
		s, _ := r.Resolve(context.Background(), path)
		fmt.Printf("%s: %s (%s)\n", path, s.Value, s.Source)
	}
	// Output:
	// gitlab.token: from-keyring (OS keyring)
	// jira.token: from-config (plain text)
}

func ExampleResolveRef() {
	defer keychain.MockInit()()
	kc := keychain.New("mytool")
	_ = kc.Set("jira.token", "s3cret")

	// values as read from a config file
	for _, value := range []string{"keychain:jira.token", "https://jira.example.com"} {
		v, _ := keychain.ResolveRef(context.Background(), kc, value)
		fmt.Println(v)
	}
	// Output:
	// s3cret
	// https://jira.example.com
}
