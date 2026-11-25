package app

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What the collector may and may not touch.
//
// The same scene tests/integration/clean_test.sh builds against a real daemon, held in memory:
// a live environment, a dead one, a project whose containers were removed but whose volume
// remains, a frozen template for each, and two branch-environment registrations — one whose
// worktree is gone and one whose worktree is still there. Every fake records what it was asked,
// because this command deletes and the tests that matter are the ones proving it leaves things
// alone.
//

const (
	alive = "hm-clean-alive"
	dead  = "hm-clean-dead"
	stray = "hm-clean-stray"

	aliveRoot = "/lab/alive-root"
	goneRoot  = "/lab/gone-root"
)

// ledger is every call made on any fake, in order, by name.
type ledger struct{ calls []string }

func (l *ledger) record(name string) { l.calls = append(l.calls, name) }

type sweepEngine struct {
	containers []core.Container
	log        *ledger
	removed    []string
}

func (e *sweepEngine) Containers() ([]core.Container, error) {
	e.log.record("Containers")

	return e.containers, nil
}

func (e *sweepEngine) Remove(ids []string) error {
	e.log.record("Remove")
	e.removed = append(e.removed, ids...)

	return nil
}

func (e *sweepEngine) Stop(ids []string) ([]string, error) {
	e.log.record("Stop")

	return nil, nil
}

type sweepVolumes struct {
	names   []string
	labels  map[string]map[string]string
	log     *ledger
	removed []string
}

func (v *sweepVolumes) Labelled(string) ([]map[string]string, error) {
	v.log.record("Labelled")

	return nil, nil
}

func (v *sweepVolumes) Exists(name string) bool {
	v.log.record("Exists")

	return v.labels[name] != nil
}

func (v *sweepVolumes) Labels(name string) map[string]string {
	v.log.record("Labels")

	return v.labels[name]
}

func (v *sweepVolumes) Create(string, map[string]string) error {
	v.log.record("Create")

	return nil
}

func (v *sweepVolumes) Remove(name string) error {
	v.log.record("Remove")
	v.removed = append(v.removed, name)

	return nil
}

func (v *sweepVolumes) All() ([]string, error) {
	v.log.record("All")

	return v.names, nil
}

func (v *sweepVolumes) Users(string) ([]string, error) {
	v.log.record("Users")

	return nil, nil
}

type sweepRegistry struct {
	parents   []string
	worktrees map[string][]core.Worktree
	log       *ledger
	forgotten []string
}

func (r *sweepRegistry) Worktree(string, string) (*core.Worktree, error) {
	r.log.record("Worktree")

	return nil, nil
}

func (r *sweepRegistry) Worktrees(parent string) ([]core.Worktree, error) {
	r.log.record("Worktrees")

	return r.worktrees[parent], nil
}

func (r *sweepRegistry) Forget(parent, name string) error {
	r.log.record("Forget")
	r.forgotten = append(r.forgotten, parent+"/"+name)

	return nil
}

func (r *sweepRegistry) Overlay(string, string) string { return "" }

func (r *sweepRegistry) WriteOverlay(string, string, string) error {
	r.log.record("WriteOverlay")

	return nil
}

func (r *sweepRegistry) Save(string, core.Worktree) error {
	r.log.record("Save")

	return nil
}

func (r *sweepRegistry) Parents() ([]string, error) {
	r.log.record("Parents")

	return r.parents, nil
}

type sweepMachine struct{ marked []string }

func (sweepMachine) MemoryBytes() int64                  { return 0 }
func (sweepMachine) FreeDiskGB() (int, bool)             { return 0, false }
func (sweepMachine) Listening() ([]core.Listener, error) { return nil, nil }
func (m sweepMachine) MarkedHosts(string) []string       { return m.marked }
func (sweepMachine) InGroup(string) bool                 { return false }
func (sweepMachine) Mkcert() (bool, string)              { return false, "" }
func (sweepMachine) HostsEntry(string) bool              { return false }
func (sweepMachine) HostsMalformed(string) bool          { return false }
func (sweepMachine) ResolvesLocally(string) bool         { return false }

type sweep struct {
	collector Collector
	engine    *sweepEngine
	volumes   *sweepVolumes
	registry  *sweepRegistry
	log       *ledger
}

func sweepOf(containers []core.Container, names []string, labels map[string]map[string]string,
	worktrees map[string][]core.Worktree, present map[string]bool, marked []string,
) sweep {
	log := &ledger{}
	parents := []string{}

	for parent := range worktrees {
		parents = append(parents, parent)
	}

	sort.Strings(parents)

	s := sweep{
		engine:   &sweepEngine{containers: containers, log: log},
		volumes:  &sweepVolumes{names: names, labels: labels, log: log},
		registry: &sweepRegistry{parents: parents, worktrees: worktrees, log: log},
		log:      log,
	}

	s.collector = Collector{
		Engine: s.engine, Volumes: s.volumes, Registry: s.registry,
		FS: filesystem{present: present}, Machine: sweepMachine{marked: marked},
		Marker: "added by hm",
	}

	return s
}

func composeLabels(project string) map[string]string {
	return map[string]string{"com.docker.compose.project": project}
}

func templateLabels(project, root string) map[string]string {
	return map[string]string{core.TemplateLabel: "base", "hm.project": project, "hm.root": root}
}

// scene is clean_test.sh's fixture, plus a container and a volume for the live branch
// environment, so that Collect has something of a live worktree it could wrongly delete.
func scene() sweep {
	return sweepOf(
		[]core.Container{
			{ID: "alive-php", ComposeProject: alive, ComposeService: "phpfpm", Project: alive, Root: aliveRoot, Running: true},
			{ID: "dead-php", ComposeProject: dead, ComposeService: "phpfpm", Project: dead, Root: goneRoot, Running: true},
			{ID: "rama-viva-php", ComposeProject: alive + "-rama-viva", ComposeService: "phpfpm", Project: alive + "-rama-viva", Root: aliveRoot},
		},
		[]string{
			"hm-template-" + alive + "-base",
			"hm-template-" + dead + "-base",
			stray + "_workspace",
			dead + "_workspace",
			alive + "-rama-viva_workspace",
			alive + "_workspace",
		},
		map[string]map[string]string{
			alive + "_workspace":             composeLabels(alive),
			dead + "_workspace":              composeLabels(dead),
			stray + "_workspace":             composeLabels(stray),
			alive + "-rama-viva_workspace":   composeLabels(alive + "-rama-viva"),
			"hm-template-" + dead + "-base":  templateLabels(dead, goneRoot),
			"hm-template-" + alive + "-base": templateLabels(alive, aliveRoot),
		},
		map[string][]core.Worktree{
			"padre": {
				{Name: "rama-muerta", Project: dead + "-rama-muerta", Path: "/lab/no-existe"},
				{Name: "rama-viva", Project: alive + "-rama-viva", Path: aliveRoot},
			},
		},
		map[string]bool{aliveRoot: true},
		[]string{alive + ".test", dead + ".test"},
	)
}

func surveyed(t *testing.T, s sweep) core.Collection {
	t.Helper()

	collection, err := s.collector.Survey()
	if err != nil {
		t.Fatalf("Survey() = %v, want no error", err)
	}

	return collection
}

func names(environments []core.Abandoned) []string {
	listed := []string{}

	for _, one := range environments {
		listed = append(listed, one.Name)
	}

	return listed
}

func has(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------- what it sees

func TestAnEnvironmentWhoseDirectoryIsGoneIsCollectable(t *testing.T) {
	collection := surveyed(t, scene())

	want := []core.Abandoned{{Name: dead, Root: goneRoot}}
	if diff := cmp.Diff(want, collection.Environments); diff != "" {
		t.Fatalf("environments (-want +got):\n%s", diff)
	}
}

func TestAnEnvironmentWhoseDirectoryExistsNeverIs(t *testing.T) {
	collection := surveyed(t, scene())

	for _, name := range names(collection.Environments) {
		if strings.HasPrefix(name, alive) {
			t.Fatalf("environments = %v, want no live environment among them", names(collection.Environments))
		}
	}
}

func TestTheVolumesOfACollectableEnvironmentComeWithIt(t *testing.T) {
	collection := surveyed(t, scene())

	if !has(collection.Volumes, dead+"_workspace") {
		t.Fatalf("volumes = %v, want %s_workspace", collection.Volumes, dead)
	}
}

func TestTheVolumesOfALiveEnvironmentDoNot(t *testing.T) {
	collection := surveyed(t, scene())

	for _, volume := range []string{alive + "_workspace", alive + "-rama-viva_workspace"} {
		if has(collection.Volumes, volume) || has(collection.Unattributable.Volumes, volume) {
			t.Fatalf("volumes = %v, unattributable = %v, want %s in neither",
				collection.Volumes, collection.Unattributable.Volumes, volume)
		}
	}
}

func TestATemplateOfAProjectThatIsGoneIsCollectable(t *testing.T) {
	collection := surveyed(t, scene())

	if !has(collection.Volumes, "hm-template-"+dead+"-base") {
		t.Fatalf("volumes = %v, want the dead project's template", collection.Volumes)
	}
}

func TestATemplateOfAProjectThatIsStillThereIsNot(t *testing.T) {
	collection := surveyed(t, scene())

	if has(collection.Volumes, "hm-template-"+alive+"-base") {
		t.Fatalf("volumes = %v, want the live project's template left alone", collection.Volumes)
	}
}

func TestATemplateIsNotReportedAsUnattributableEither(t *testing.T) {
	collection := surveyed(t, scene())

	for _, volume := range collection.Unattributable.Volumes {
		if strings.HasPrefix(volume, "hm-template-") {
			t.Fatalf("unattributable volumes = %v, want no template among them", collection.Unattributable.Volumes)
		}
	}
}

func TestAVolumeWithNoContainersIsListedAsUnattributable(t *testing.T) {
	collection := surveyed(t, scene())

	if diff := cmp.Diff([]string{stray + "_workspace"}, collection.Unattributable.Volumes); diff != "" {
		t.Fatalf("unattributable volumes (-want +got):\n%s", diff)
	}

	if has(collection.Volumes, stray+"_workspace") {
		t.Fatalf("volumes = %v, want the unattributable one only listed, never collectable", collection.Volumes)
	}
}

// A volume created outside Compose carries no project label at all and is skipped — neither
// collectable nor listed — because this tool never creates one that way.
func TestAVolumeWithNoLabelsIsSkipped(t *testing.T) {
	s := sweepOf(nil, []string{"somebody-elses"}, map[string]map[string]string{"somebody-elses": {}},
		nil, map[string]bool{}, nil)

	collection := surveyed(t, s)

	if len(collection.Volumes) != 0 || len(collection.Unattributable.Volumes) != 0 {
		t.Fatalf("volumes = %v, unattributable = %v, want the unlabelled volume in neither",
			collection.Volumes, collection.Unattributable.Volumes)
	}
}

func TestABranchEnvironmentWhoseWorktreeIsGoneIsCollectable(t *testing.T) {
	collection := surveyed(t, scene())

	want := []core.AbandonedWorktree{{Name: "padre/rama-muerta", Project: dead + "-rama-muerta", Path: "/lab/no-existe"}}
	if diff := cmp.Diff(want, collection.Worktrees); diff != "" {
		t.Fatalf("worktrees (-want +got):\n%s", diff)
	}
}

func TestLookingDeletesNothing(t *testing.T) {
	s := scene()
	collection := surveyed(t, s)

	if collection.Removed {
		t.Fatal("removed = true, want a survey to say nothing was removed")
	}

	reads := map[string]bool{"Containers": true, "All": true, "Labels": true, "Parents": true, "Worktrees": true}

	for _, name := range s.log.calls {
		if !reads[name] {
			t.Fatalf("calls = %v, want only reads: a survey asked for %s", s.log.calls, name)
		}
	}
}

// Database snapshots live as files under HM_SNAPSHOT_DIR. The collector reaches files only
// through ports.FS, which cannot delete, and a collection has nowhere to list one: the document
// carries exactly these keys, so a snapshot list added to it fails here first.
func TestDatabaseSnapshotsAreNotPartOfAClean(t *testing.T) {
	body, err := json.Marshal(surveyed(t, scene()))
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}

	keys := []string{}
	for key := range document {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	want := []string{"environments", "hosts", "removed", "unattributable", "volumes", "worktrees"}
	if diff := cmp.Diff(want, keys); diff != "" {
		t.Fatalf("collection keys (-want +got):\n%s", diff)
	}

	field, _ := reflect.TypeOf(Collector{}).FieldByName("FS")
	files := field.Type

	for index := 0; index < files.NumMethod(); index++ {
		if name := files.Method(index).Name; name != "IsDir" && name != "Exists" && name != "Read" && name != "MkdirAll" {
			t.Fatalf("ports.FS has %s, want no way for the collector to delete a file", name)
		}
	}
}

// `docker system prune` is the tool this command exists to not be. A structural pin rather than a
// runtime one: no port the collector holds has a prune to call.
func TestTheCollectorCanInvokeNoPrune(t *testing.T) {
	collector := reflect.TypeOf(Collector{})

	for index := 0; index < collector.NumField(); index++ {
		port := collector.Field(index).Type
		if port.Kind() != reflect.Interface {
			continue
		}

		for method := 0; method < port.NumMethod(); method++ {
			if strings.Contains(strings.ToLower(port.Method(method).Name), "prune") {
				t.Fatalf("%s.%s: want no prune reachable from the collector",
					collector.Field(index).Name, port.Method(method).Name)
			}
		}
	}
}

func TestNothingToCollectIsAnEmptyCollectionAndNoError(t *testing.T) {
	s := sweepOf(
		[]core.Container{{ID: "a", ComposeProject: alive, ComposeService: "phpfpm", Project: alive, Root: aliveRoot}},
		[]string{alive + "_workspace"},
		map[string]map[string]string{alive + "_workspace": composeLabels(alive)},
		map[string][]core.Worktree{"padre": {{Name: "rama-viva", Project: alive + "-rama-viva", Path: aliveRoot}}},
		map[string]bool{aliveRoot: true},
		nil,
	)

	collection := surveyed(t, s)

	if collection.Anything() {
		t.Fatalf("Anything() = true for %+v, want nothing to collect", collection)
	}

	// Empty lists and not nil ones: the document says [] and not null
	if collection.Environments == nil || collection.Volumes == nil || collection.Worktrees == nil ||
		collection.Hosts == nil || collection.Unattributable.Volumes == nil ||
		collection.Unattributable.Environments == nil {
		t.Fatalf("collection = %#v, want every list empty and present", collection)
	}

	if len(collection.Environments)+len(collection.Volumes)+len(collection.Worktrees)+
		len(collection.Hosts)+len(collection.Unattributable.Volumes)+len(collection.Unattributable.Environments) != 0 {
		t.Fatalf("collection = %+v, want every list empty", collection)
	}
}

// Templates outlive their project's containers — they are frozen data, not part of a compose
// project — so a machine can be left with nothing of a dead project but its template. That alone is
// something to collect, and collecting it deletes the template and nothing else.
func TestAMachineWithOnlyADeadTemplateHasSomethingToCollect(t *testing.T) {
	template := "hm-template-" + dead + "-base"
	s := sweepOf(
		nil,
		[]string{template},
		map[string]map[string]string{template: templateLabels(dead, goneRoot)},
		map[string][]core.Worktree{},
		map[string]bool{},
		nil,
	)

	collection := surveyed(t, s)

	if !collection.Anything() {
		t.Fatalf("Anything() = false for %+v, want the dead template to be something to collect", collection)
	}

	if err := s.collector.Collect(collection); err != nil {
		t.Fatalf("Collect() = %v, want no error", err)
	}

	if diff := cmp.Diff([]string{template}, s.volumes.removed); diff != "" {
		t.Fatalf("volumes removed (-want +got):\n%s", diff)
	}

	if len(s.engine.removed) != 0 || len(s.registry.forgotten) != 0 {
		t.Fatalf("containers removed = %v, forgotten = %v, want neither", s.engine.removed, s.registry.forgotten)
	}
}

// What is listed but never removed — hosts entries, unattributable volumes and environments — is
// not something to collect on its own: --force has nothing to delete.
func TestOnlyWhatIsNeverRemovedIsNothingToCollect(t *testing.T) {
	collection := core.Collection{Hosts: []string{dead + ".test"}}
	collection.Unattributable.Volumes = []string{stray + "_workspace"}
	collection.Unattributable.Environments = []core.Abandoned{{Name: "viejo", Reason: "no hm labels"}}

	if collection.Anything() {
		t.Fatalf("Anything() = true for %+v, want nothing to collect", collection)
	}
}

// The lists are separate lists, each sorted, whatever order Docker and the registry answered in.
func TestTheListsAreListsAndSorted(t *testing.T) {
	s := sweepOf(
		[]core.Container{
			{ID: "z", ComposeProject: "zeta", ComposeService: "phpfpm", Project: "zeta", Root: "/gone/zeta"},
			{ID: "a", ComposeProject: "alfa", ComposeService: "phpfpm", Project: "alfa", Root: "/gone/alfa"},
			{ID: "m", ComposeProject: "mu", ComposeService: "phpfpm", Project: "mu", Root: "/gone/mu"},
			{ID: "u2", ComposeProject: "viejo2", ComposeService: "phpfpm"},
			{ID: "u1", ComposeProject: "viejo1", ComposeService: "phpfpm"},
		},
		[]string{"zeta_db", "alfa_db", "solo2_db", "solo1_db", "mu_db"},
		map[string]map[string]string{
			"zeta_db": composeLabels("zeta"), "alfa_db": composeLabels("alfa"), "mu_db": composeLabels("mu"),
			"solo2_db": composeLabels("solo2"), "solo1_db": composeLabels("solo1"),
		},
		map[string][]core.Worktree{
			"padre": {{Name: "zz", Project: "padre-zz", Path: "/gone/zz"}, {Name: "aa", Project: "padre-aa", Path: "/gone/aa"}},
		},
		map[string]bool{},
		[]string{"zz.test", "aa.test"},
	)

	collection := surveyed(t, s)

	want := core.Collection{
		Environments: []core.Abandoned{
			{Name: "alfa", Root: "/gone/alfa"}, {Name: "mu", Root: "/gone/mu"}, {Name: "zeta", Root: "/gone/zeta"},
		},
		Volumes: []string{"alfa_db", "mu_db", "zeta_db"},
		Worktrees: []core.AbandonedWorktree{
			{Name: "padre/aa", Project: "padre-aa", Path: "/gone/aa"},
			{Name: "padre/zz", Project: "padre-zz", Path: "/gone/zz"},
		},
		Hosts: []string{"aa.test", "zz.test"},
	}
	want.Unattributable.Volumes = []string{"solo1_db", "solo2_db"}
	want.Unattributable.Environments = []core.Abandoned{
		{Name: "viejo1", Reason: "no hm labels"}, {Name: "viejo2", Reason: "no hm labels"},
	}

	if diff := cmp.Diff(want, collection); diff != "" {
		t.Fatalf("collection (-want +got):\n%s", diff)
	}
}

// ---------------------------------------------------------------- collecting

func TestCollectDeletesWhatTheSurveyFoundAndNothingLive(t *testing.T) {
	s := scene()

	if err := s.collector.Collect(surveyed(t, s)); err != nil {
		t.Fatalf("Collect() = %v, want no error", err)
	}

	if diff := cmp.Diff([]string{"dead-php"}, s.engine.removed); diff != "" {
		t.Fatalf("containers removed (-want +got):\n%s", diff)
	}

	removed := append([]string{}, s.volumes.removed...)
	sort.Strings(removed)

	if diff := cmp.Diff([]string{dead + "_workspace", "hm-template-" + dead + "-base"}, removed); diff != "" {
		t.Fatalf("volumes removed (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"padre/rama-muerta"}, s.registry.forgotten); diff != "" {
		t.Fatalf("registrations forgotten (-want +got):\n%s", diff)
	}

	for _, call := range s.log.calls {
		if call == "Stop" || call == "Create" || call == "Save" || call == "WriteOverlay" {
			t.Fatalf("calls = %v, want nothing but reads, removals and forgetting", s.log.calls)
		}
	}
}

func TestCollectNeverDeletesAnUnattributableVolume(t *testing.T) {
	s := scene()
	collection := surveyed(t, s)

	if len(collection.Unattributable.Volumes) == 0 {
		t.Fatal("unattributable volumes = none, want the stray one so this test means something")
	}

	if err := s.collector.Collect(collection); err != nil {
		t.Fatal(err)
	}

	for _, volume := range collection.Unattributable.Volumes {
		if has(s.volumes.removed, volume) {
			t.Fatalf("volumes removed = %v, want %s to survive: that is the whole point", s.volumes.removed, volume)
		}
	}
}

func TestCollectLeavesALiveWorktreeAlone(t *testing.T) {
	s := scene()

	if err := s.collector.Collect(surveyed(t, s)); err != nil {
		t.Fatal(err)
	}

	if has(s.registry.forgotten, "padre/rama-viva") {
		t.Fatalf("forgotten = %v, want the live registration left alone", s.registry.forgotten)
	}

	if has(s.engine.removed, "rama-viva-php") || has(s.volumes.removed, alive+"-rama-viva_workspace") {
		t.Fatalf("removed containers %v, volumes %v, want nothing of the live branch environment",
			s.engine.removed, s.volumes.removed)
	}
}

// Collect does not survey again: what it deletes is what it was handed, even while the machine
// holds more that a survey would find.
func TestCollectDeletesOnlyWhatTheCollectionLists(t *testing.T) {
	s := scene()

	if err := s.collector.Collect(core.Collection{Volumes: []string{dead + "_workspace"}}); err != nil {
		t.Fatal(err)
	}

	if len(s.engine.removed) != 0 || len(s.registry.forgotten) != 0 {
		t.Fatalf("removed containers %v, forgotten %v, want nothing that was not listed",
			s.engine.removed, s.registry.forgotten)
	}

	if diff := cmp.Diff([]string{dead + "_workspace"}, s.volumes.removed); diff != "" {
		t.Fatalf("volumes removed (-want +got):\n%s", diff)
	}
}

// TestAnEmptyNameDeletesNothingByName: Collect deletes a branch environment's containers and
// volumes by its compose project name, and a dead environment's containers by its hm.project name.
// An empty name would match every container with no such label and every volume starting with "_"
// — things that belong to nobody this tool knows. With no name there is nothing to match, so
// nothing is deleted by name; a stale registration is still forgotten.
func TestAnEmptyNameDeletesNothingByName(t *testing.T) {
	s := sweepOf(
		[]core.Container{
			{ID: "someone-elses", Running: true},
			{ID: "unlabelled-compose", ComposeProject: "", Project: ""},
		},
		[]string{"_scratch", "_data"},
		map[string]map[string]string{},
		map[string][]core.Worktree{"padre": {{Name: "rota", Project: "", Path: "/lab/no-existe"}}},
		map[string]bool{},
		nil,
	)

	collection := core.Collection{
		Environments: []core.Abandoned{{Name: ""}},
		Worktrees:    []core.AbandonedWorktree{{Name: "padre/rota", Project: "", Path: "/lab/no-existe"}},
	}

	if err := s.collector.Collect(collection); err != nil {
		t.Fatal(err)
	}

	if len(s.engine.removed) != 0 || len(s.volumes.removed) != 0 {
		t.Fatalf("removed containers %v, volumes %v, want nothing matched by an empty name",
			s.engine.removed, s.volumes.removed)
	}

	if !has(s.registry.forgotten, "padre/rota") {
		t.Errorf("forgotten = %v, want the stale registration forgotten all the same", s.registry.forgotten)
	}
}

// TestABranchEnvironmentsVolumesAreTheOnesComposeLabelledForIt: compose project names may contain
// underscores, so a dead branch environment called "x" and a live project called "x_algo" share
// the prefix "x_". Matching by prefix took the live project's volumes with the dead one's; matching
// by the compose project label takes only the dead one's.
func TestABranchEnvironmentsVolumesAreTheOnesComposeLabelledForIt(t *testing.T) {
	s := sweepOf(
		nil,
		[]string{"x_data", "x_algo_data", "x_unlabelled"},
		map[string]map[string]string{
			"x_data":      composeLabels("x"),
			"x_algo_data": composeLabels("x_algo"),
		},
		map[string][]core.Worktree{"padre": {{Name: "x", Project: "x", Path: "/lab/no-existe"}}},
		map[string]bool{},
		nil,
	)

	collection := core.Collection{
		Worktrees: []core.AbandonedWorktree{{Name: "padre/x", Project: "x", Path: "/lab/no-existe"}},
	}

	if err := s.collector.Collect(collection); err != nil {
		t.Fatal(err)
	}

	if !has(s.volumes.removed, "x_data") {
		t.Errorf("removed %v, want the dead branch environment's own volume", s.volumes.removed)
	}

	if has(s.volumes.removed, "x_algo_data") || has(s.volumes.removed, "x_unlabelled") {
		t.Errorf("removed %v, want nothing Compose did not label for this branch environment", s.volumes.removed)
	}
}
