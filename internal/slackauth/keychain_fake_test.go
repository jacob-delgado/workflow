// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

import (
	"context"
	"os/user"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// keychainUser is the user a fake keychain's items are kept for.
const keychainUser = "jacob"

// keychainByService is macOS's security keeping a secret under each service:
// it keeps what an add line on its input hands it, answers a find with what
// it keeps, and forgets what a delete names, exiting 44 for a service it
// keeps nothing under. A store or a delete exits with storing or deleting
// where that is not zero; done is each store and delete it made, in order.
type keychainByService struct {
	lock              sync.Mutex
	kept              map[string]string
	storing, deleting int
	done              []string
}

// run is security, run as the item runs it.
func (k *keychainByService) run(_ context.Context, program proc.Command, input []byte) ([]byte, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	if program.Args[0] == "-i" {
		return nil, k.store(string(input))
	}

	service := program.Args[4]

	secret, ok := k.kept[service]
	if !ok {
		return nil, exitedWith(securityItemNotFound)
	}

	if program.Args[0] == "find-generic-password" {
		return []byte(secret + "\n"), nil
	}

	if k.deleting != 0 {
		return nil, exitedWith(k.deleting)
	}

	delete(k.kept, service)
	k.done = append(k.done, "deleted "+service)

	return nil, nil
}

// store keeps the secret line hands it under line's service.
func (k *keychainByService) store(line string) error {
	if k.storing != 0 {
		return exitedWith(k.storing)
	}

	service := line[strings.Index(line, ` -s "`)+len(` -s "`) : strings.Index(line, `" -w "`)]
	secret := line[strings.Index(line, ` -w "`)+len(` -w "`):]
	k.kept[service] = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(strings.TrimSuffix(secret, "\"\n"))
	k.done = append(k.done, "stored "+service)

	return nil
}

// services are the services the keychain keeps something under.
func (k *keychainByService) services() []string {
	k.lock.Lock()
	defer k.lock.Unlock()

	names := make([]string, 0, len(k.kept))
	for name := range k.kept {
		names = append(names, name)
	}

	return names
}

// made is each store and delete the keychain made, in order.
func (k *keychainByService) made() []string {
	k.lock.Lock()
	defer k.lock.Unlock()

	return slices.Clone(k.done)
}

// item is the Slack token's item over the keychain.
func (k *keychainByService) item(t *testing.T) keychain.Item {
	t.Helper()

	item, ok := keychain.Open("darwin", slackauth.KeychainService, k.run,
		func() (*user.User, error) { return &user.User{Username: keychainUser}, nil }, func(string) string { return "" })
	if !ok {
		t.Fatal("no keychain item on darwin")
	}

	return item
}

// securityItemNotFound is the status security exits with for an item it does not keep.
const securityItemNotFound = 44

// exitedWith is security exiting with code.
func exitedWith(code int) error {
	return &proc.ExitError{Program: "security", Code: code, Stderr: "security refused"}
}

// emptyKeychain is the Slack token's item over a keychain that keeps
// nothing yet.
func emptyKeychain(t *testing.T) keychain.Item {
	t.Helper()

	return (&keychainByService{kept: map[string]string{}}).item(t)
}
