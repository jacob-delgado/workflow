// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// The directories the cases work in, under anaHome.
const (
	anaHome   = "/home/ana"
	apiRoot   = "/home/ana/src/api"
	apiCmd    = "/home/ana/src/api/cmd"
	webRoot   = "/home/ana/src/web"
	notesDir  = "/home/ana/notes"
	oldDir    = "/home/ana/old"
	reposPath = "/api/repositories"
)

// apiPlace is the api repository, worked in from its cmd directory.
func apiPlace() seams.Place {
	return seams.Place{
		Dir: apiCmd, Root: apiRoot,
		Remote: forge.Repo{Kind: forge.KindGitHub, Host: "github.com", Path: "acme/api"},
		Config: config.Files{Repo: apiRoot + "/.workflow.json", Home: anaHome + "/.workflow.json"},
	}
}

// placesByDir are the directories there are, as Look reads them.
func placesByDir() map[string]seams.Place {
	return map[string]seams.Place{
		apiCmd:   apiPlace(),
		webRoot:  {Dir: webRoot, Root: webRoot, Remote: forge.Repo{Host: "github.com", Path: "acme/web"}},
		notesDir: {Dir: notesDir},
	}
}

// keptFavorites is the favorites a case's store keeps, and what it was asked.
type keptFavorites struct {
	mu    sync.Mutex
	dirs  []string
	asked []string
	// hold, when set, keeps a Favor from answering until it is closed, and
	// entered is closed once one has started.
	hold, entered chan struct{}
}

// repositoryDeps work in api's cmd directory, with web, notes and a directory
// since removed as favorites.
func repositoryDeps(kept *keptFavorites) webserver.Deps {
	deps := filledDeps()
	places := placesByDir()
	deps.Repositories = seams.Repositories{
		Here: apiPlace(), Home: anaHome,
		Look: func(dir string) (seams.Place, error) {
			place, found := places[dir]
			if !found {
				return seams.Place{}, workdirs.ErrNotFound
			}

			return place, nil
		},
		Subdirectories: func(dir, _ string) (workdirs.Listing, error) {
			if dir != "/home/ana/src" {
				return workdirs.Listing{}, workdirs.ErrNotFound
			}

			return workdirs.Listing{Entries: []workdirs.Entry{{Name: "api", Repository: true}, {Name: "notes"}}}, nil
		},
	}
	deps.Favorites = func() ([]string, error) {
		kept.mu.Lock()
		defer kept.mu.Unlock()

		return slices.Clone(kept.dirs), nil
	}
	deps.Favor = func(dir string) error {
		if kept.hold != nil {
			close(kept.entered)
			<-kept.hold
		}

		kept.mu.Lock()
		defer kept.mu.Unlock()

		kept.asked = append(kept.asked, "favor "+dir)
		kept.dirs = append(kept.dirs, dir)

		return nil
	}
	deps.Unfavor = func(dir string) error {
		kept.mu.Lock()
		defer kept.mu.Unlock()

		kept.asked = append(kept.asked, "unfavor "+dir)

		return nil
	}

	return deps
}

// favoritesKept is a store keeping web, notes and a removed directory.
func favoritesKept() *keptFavorites {
	return &keptFavorites{dirs: []string{webRoot, notesDir, oldDir, apiCmd}}
}

func TestRepositoriesSayWhereTheServerWorks(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, repositoryDeps(favoritesKept()), config.Default()), reposPath)

	// Assert
	got := decode[api.Repositories](t, recorder)
	want := api.Place{
		Dir: apiCmd, Shown: "~/src/api/cmd", Root: apiRoot, RootShown: "~/src/api", Within: "cmd",
		Origin: "github.com/acme/api", Config: []string{"~/src/api/.workflow.json", "~/.workflow.json"},
	}

	if recorder.Code != http.StatusOK || !samePlace(got.Here, want) || !got.FavoritesKept {
		t.Errorf("status %d, here %+v (kept: %v); want %+v and favorites kept", recorder.Code, got.Here,
			got.FavoritesKept, want)
	}
}

// samePlace reports two places alike, field by field.
func samePlace(one, other api.Place) bool {
	return one.Dir == other.Dir && one.Shown == other.Shown && one.Root == other.Root &&
		one.RootShown == other.RootShown && one.Within == other.Within && one.Origin == other.Origin &&
		slices.Equal(one.Config, other.Config)
}

func TestFavoritesSayWhatIsThereNow(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, repositoryDeps(favoritesKept()), config.Default()), reposPath)

	// Assert
	got := decode[api.Repositories](t, recorder)

	states := map[string]api.FavoriteState{}
	for _, favorite := range got.Favorites {
		states[favorite.Shown] = favorite.State
	}

	want := map[string]api.FavoriteState{
		"~/src/web": api.FavoriteRepository, "~/notes": api.FavoriteDirectory,
		"~/old": api.FavoriteMissing, "~/src/api/cmd": api.FavoriteHere,
	}
	if len(states) != len(want) {
		t.Fatalf("favorites %+v, want %v", got.Favorites, want)
	}

	for shown, state := range want {
		if states[shown] != state {
			t.Errorf("%s is %q, want %q", shown, states[shown], state)
		}
	}
}

func TestAStoreThatKeepsNothingSaysFavoritesAreNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := repositoryDeps(favoritesKept())
	deps.Favor, deps.Unfavor = nil, nil
	handler := serve(t, deps, config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)

	// Assert
	kept := decode[api.Repositories](t, get(t, handler, reposPath)).FavoritesKept
	if recorder.Code != http.StatusUnprocessableEntity || kept {
		t.Errorf("status %d; want 422 and favorites_kept false", recorder.Code)
	}
}

func TestAFavoriteIsMarkedAndForgotten(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, target, body, want string
	}{
		"marked": {
			method: http.MethodPut, target: reposPath + "/favorites", body: `{"dir":"/srv/web"}`, want: "favor /srv/web",
		},
		"forgotten": {
			method: http.MethodDelete, target: reposPath + "/favorites?dir=%2Fsrv%2Fweb", want: "unfavor /srv/web",
		},
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept := favoritesKept()
			handler := serve(t, repositoryDeps(kept), config.Default())

			// Act
			recorder := send(t, handler, change.method, change.target, change.body)

			// Assert
			if recorder.Code != http.StatusOK || !slices.Equal(kept.asked, []string{change.want}) {
				t.Errorf("status %d, store asked %v; want 200 and %q", recorder.Code, kept.asked, change.want)
			}
		})
	}
}

func TestAFavoriteIsAnAbsolutePath(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := favoritesKept()
	handler := serve(t, repositoryDeps(kept), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"src/web"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || len(kept.asked) != 0 {
		t.Errorf("status %d, store asked %v; want 422 and nothing kept", recorder.Code, kept.asked)
	}
}

func TestTheDirectoriesInOneAreListed(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, repositoryDeps(favoritesKept()), config.Default())

	// Act
	// A path is read from your home after a ~.
	recorder := get(t, handler, "/api/directories?path=~%2Fsrc")

	// Assert
	got := decode[api.DirectoryListing](t, recorder)
	if recorder.Code != http.StatusOK || got.Path != "/home/ana/src" || got.Shown != "~/src" ||
		got.Parent != anaHome || len(got.Entries) != 2 || got.Entries[0].Path != apiRoot || !got.Entries[0].Repository {
		t.Errorf("status %d, listing %+v; want ~/src with api, a repository, first", recorder.Code, got)
	}
}

func TestADirectoryNotThereIsRefusedWithoutItsPath(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, repositoryDeps(favoritesKept()), config.Default())

	// Act
	recorder := get(t, handler, "/api/directories?path=%2Fsecret%2Fplace")

	// Assert
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "secret") {
		t.Errorf("status %d, body %s; want 404 that names no path", recorder.Code, recorder.Body.String())
	}
}

// reachable lets deps switch to any directory Look reads, and records each
// switch asked.
func reachable(deps webserver.Deps, reached *[]string) webserver.Deps {
	deps.Reach = func(dir string) (webserver.World, error) {
		*reached = append(*reached, dir)

		place, err := deps.Repositories.Look(dir)
		if err != nil {
			return webserver.World{}, err
		}

		next := deps
		next.Repositories.Here = place

		return webserver.World{Deps: next, Config: config.Default(), Info: webserver.Info{Version: testVersion}}, nil
	}

	return deps
}

func TestASwitchServesTheDirectorySwitchedTo(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	after := decode[api.Repositories](t, get(t, handler, reposPath))
	if recorder.Code != http.StatusOK || after.Here.Dir != webRoot || !slices.Equal(reached, []string{webRoot}) {
		t.Errorf("status %d, now in %q, reached %v; want 200 and web served", recorder.Code, after.Here.Dir, reached)
	}
}

func TestASwitchToADirectoryNotThereIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"/secret/gone"}`)

	// Assert
	after := decode[api.Repositories](t, get(t, handler, reposPath))

	named := strings.Contains(recorder.Body.String(), "secret")
	if recorder.Code != http.StatusNotFound || after.Here.Dir != apiCmd || named {
		t.Errorf("status %d, body %s, now in %q; want 404 naming no path, still in api", recorder.Code,
			recorder.Body.String(), after.Here.Dir)
	}
}

func TestASwitchWaitsForNoWriteButIsRefusedWhileOneRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	// A favorite being marked is a write the switch would leave unknown.
	var reached []string

	kept := favoritesKept()
	kept.hold, kept.entered = make(chan struct{}), make(chan struct{})
	handler := serve(t, reachable(repositoryDeps(kept), &reached), config.Default())

	writing := make(chan struct{})
	go func() {
		defer close(writing)

		send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)
	}()

	<-kept.entered

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	close(kept.hold)
	<-writing

	if recorder.Code != http.StatusConflict || len(reached) != 0 {
		t.Errorf("status %d, reached %v; want 409 and no switch", recorder.Code, reached)
	}
}

func TestAWriteFromAPageShowingAnotherDirectoryIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		shown string
		want  int
	}{
		"the directory the server works in": {shown: apiCmd, want: http.StatusOK},
		"another":                           {shown: webRoot, want: http.StatusConflict},
		// A header carries bytes, not text, so the page escapes the path as a
		// URL path is escaped.
		"escaped":          {shown: "%2Fhome%2Fana%2Fsrc%2Fapi%2Fcmd", want: http.StatusOK},
		"escaped, another": {shown: "%2Fhome%2Fjos%C3%A9", want: http.StatusConflict},
	}

	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			handler := serve(t, repositoryDeps(favoritesKept()), config.Default())
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, reposPath+"/favorites",
				strings.NewReader(`{"dir":"/srv/web"}`))
			request.Host = loopbackHost
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(webserver.HereHeader, page.shown)

			recorder := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(recorder, request)

			// Assert
			if recorder.Code != page.want {
				t.Errorf("status %d, want %d", recorder.Code, page.want)
			}
		})
	}
}

func TestASwitchEndsTheEventStreamSoThePageReconnects(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	handler := serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	lines := bufio.NewScanner(response.Body)
	for lines.Scan() && !strings.HasPrefix(lines.Text(), "data: ") {
		continue
	}

	// Act
	send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"`+webRoot+`"}`)

	// Assert
	var rest []string
	for lines.Scan() {
		rest = append(rest, lines.Text())
	}

	if !slices.Contains(rest, "retry: 1000") {
		t.Errorf("the stream went on with %q, want it to end asking the page to reconnect", rest)
	}
}

func TestEverySnapshotSaysWhereTheServerWorks(t *testing.T) {
	t.Parallel()

	// Act
	recorder := streamOnce(t, serve(t, repositoryDeps(favoritesKept()), config.Default()), "/api/events")

	// Assert
	if here := firstSnapshot(t, recorder.Body.String()).Here; here != apiCmd {
		t.Errorf("snapshot here = %q, want %s", here, apiCmd)
	}
}

func TestAFavoriteInKeptDataFromAnotherBuildSaysHowToStartItFresh(t *testing.T) {
	t.Parallel()

	// Arrange
	// Favorites came with the kept schema's second version, so a kept file an
	// earlier build made refuses one until it is started fresh.
	deps := repositoryDeps(favoritesKept())
	deps.Favor = func(string) error { return store.ErrKeptSchemaDiffers }
	handler := serve(t, deps, config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/favorites", `{"dir":"/srv/web"}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), "db-clean --all") {
		t.Errorf("status %d, body %s; want 422 naming workflow db-clean --all", recorder.Code, recorder.Body.String())
	}
}

func TestAFavoriteKeptThroughALinkToWhereTheServerWorksIsHere(t *testing.T) {
	t.Parallel()

	// Arrange
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")

	err := os.Symlink(target, link)
	if err != nil {
		t.Fatal(err)
	}

	deps := repositoryDeps(&keptFavorites{dirs: []string{link}})
	deps.Repositories.Here = seams.Place{Dir: target}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), reposPath)

	// Assert
	got := decode[api.Repositories](t, recorder)
	if len(got.Favorites) != 1 || got.Favorites[0].State != api.FavoriteHere || got.Favorites[0].Dir != link {
		t.Errorf("favorites %+v, want the link kept as where the server works", got.Favorites)
	}
}

func TestASwitchIsToThePathWrittenPlainly(t *testing.T) {
	t.Parallel()

	// Arrange
	// Where the server works is a path written plainly, as a favorite must be.
	var reached []string

	handler := serve(t, reachable(repositoryDeps(favoritesKept()), &reached), config.Default())

	// Act
	recorder := send(t, handler, http.MethodPut, reposPath+"/here", `{"dir":"/home/ana/src/api/../web/"}`)

	// Assert
	if recorder.Code != http.StatusOK || !slices.Equal(reached, []string{webRoot}) {
		t.Errorf("status %d, reached %v; want %s", recorder.Code, reached, webRoot)
	}
}

func TestWithoutAPathTheDirectoryWorkedInIsListed(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := repositoryDeps(favoritesKept())
	deps.Repositories.Here = seams.Place{Dir: anaHome + "/src"}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/directories")

	// Assert
	if got := decode[api.DirectoryListing](t, recorder); recorder.Code != http.StatusOK || got.Path != anaHome+"/src" {
		t.Errorf("status %d, listing %+v; want where the server works", recorder.Code, got)
	}
}

func TestWhatTheServerCannotDoWithDirectoriesIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cut            func(*webserver.Deps)
		method, target string
		body           string
	}{
		"listing with no way to list": {
			cut:    func(deps *webserver.Deps) { deps.Repositories.Subdirectories = nil },
			method: http.MethodGet, target: "/api/directories",
		},
		"switching with no way to reach another": {
			cut:    func(deps *webserver.Deps) { deps.Reach = nil },
			method: http.MethodPut, target: reposPath + "/here", body: `{"dir":"` + webRoot + `"}`,
		},
	}

	for name, refused := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := repositoryDeps(favoritesKept())
			refused.cut(&deps)

			// Act
			recorder := send(t, serve(t, deps, config.Default()), refused.method, refused.target, refused.body)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d, want 422", recorder.Code)
			}
		})
	}
}

func TestTheRootHasNothingAbove(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := repositoryDeps(favoritesKept())
	deps.Repositories.Subdirectories = func(string, string) (workdirs.Listing, error) { return workdirs.Listing{}, nil }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/directories?path=%2F")

	// Assert
	if got := decode[api.DirectoryListing](t, recorder); got.Path != "/" || got.Parent != "" {
		t.Errorf("listing %+v, want / with no parent", got)
	}
}

func TestADryRunListsFavoritesAndKeepsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := repositoryDeps(favoritesKept())
	handler := serveWith(t, deps, config.Default(), webserver.Info{Version: testVersion, DryRun: true})

	// Act
	got := decode[api.Repositories](t, get(t, handler, reposPath))

	// Assert
	if got.FavoritesKept || len(got.Favorites) == 0 {
		t.Errorf("favorites %d, kept %v; want them listed and not kept", len(got.Favorites), got.FavoritesKept)
	}
}

func TestFavoritesThatCannotBeReadOrLookedAtStillAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	// A store that cannot be read lists none, and a server that cannot look
	// at a directory says it is missing.
	deps := repositoryDeps(favoritesKept())
	deps.Repositories.Look = nil
	deps.Favorites = func() ([]string, error) { return []string{webRoot}, errSeam }

	// Act
	recorder := get(t, serve(t, deps, config.Default()), reposPath)

	// Assert
	got := decode[api.Repositories](t, recorder)
	if recorder.Code != http.StatusOK || len(got.Favorites) != 1 || got.Favorites[0].State != api.FavoriteMissing {
		t.Errorf("status %d, favorites %+v; want web, missing", recorder.Code, got.Favorites)
	}
}
