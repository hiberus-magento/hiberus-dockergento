package registry

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func abierto(t *testing.T) *Store {
	t.Helper()

	store, err := Open(filepath.Join(t.TempDir(), "registro", "hm.db"))
	if err != nil {
		t.Fatalf("Open = %v, want no error", err)
	}

	t.Cleanup(func() { store.Close() })

	return store
}

func rama(nombre string) core.Worktree {
	return core.Worktree{
		Name: nombre, Branch: "feature/" + nombre, Profile: "agent",
		Project: "tienda-" + nombre, Domain: nombre + ".tienda.test",
		Path: "/code/tienda-" + nombre,
	}
}

// ---------------------------------------------------------------- lo que sustituye a los JSON

func TestUnaRamaSinRegistrarNoEsUnError(t *testing.T) {
	// Es la respuesta que más importa: distingue un worktree con entorno propio de uno que toma
	// prestada la identidad del principal, y de esa distinción dependen las negativas
	registrada, err := abierto(t).Worktree("tienda", "no-existe")
	if err != nil {
		t.Fatalf("asking about a registration that is not there = %v, want no error", err)
	}

	if registrada != nil {
		t.Fatalf("registration = %+v, want none", registrada)
	}
}

func TestRegistrarYLeerUnaRama(t *testing.T) {
	store := abierto(t)

	if err := store.Register(core.Project{Name: "tienda", Root: "/code/tienda"}, rama("azul")); err != nil {
		t.Fatalf("Register = %v, want no error", err)
	}

	registrada, err := store.Worktree("tienda", "azul")
	if err != nil || registrada == nil {
		t.Fatalf("Worktree = %v, want no error", err)
	}

	if registrada.Project != "tienda-azul" || registrada.Domain != "azul.tienda.test" {
		t.Fatalf("registration = %+v, want the name and address it recorded", registrada)
	}

	if registrada.Parent != "tienda" {
		t.Fatalf("parent = %q, want the project it belongs to", registrada.Parent)
	}
}

func TestRegistrarDosVecesActualizaEnVezDeDuplicar(t *testing.T) {
	store := abierto(t)
	proyecto := core.Project{Name: "tienda", Root: "/code/tienda"}

	primera := rama("azul")
	store.Register(proyecto, primera) //nolint:errcheck

	segunda := primera
	segunda.Branch = "otra-rama"
	segunda.SharedVendor = true

	if err := store.Register(proyecto, segunda); err != nil {
		t.Fatalf("registering it again = %v, want no error", err)
	}

	ramas, err := store.Worktrees("tienda")
	if err != nil || len(ramas) != 1 {
		t.Fatalf("worktrees = %+v (%v), want one", ramas, err)
	}

	if ramas[0].Branch != "otra-rama" || !ramas[0].SharedVendor {
		t.Fatalf("registration = %+v, want the values from the second time", ramas[0])
	}
}

func TestDosEntornosNoPuedenLlamarseIgual(t *testing.T) {
	// Dos entornos respondiendo al mismo nombre de proyecto compartirían contenedores, volúmenes
	// y base de datos sin que ninguno de los dos lo supiera
	store := abierto(t)

	uno := rama("azul")
	otro := rama("verde")
	otro.Project = uno.Project

	store.Register(core.Project{Name: "tienda"}, uno) //nolint:errcheck

	if err := store.Register(core.Project{Name: "tienda"}, otro); err == nil {
		t.Fatal("registering a name already taken = nil, want a refusal")
	}
}

func TestOlvidarUnaRama(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck

	if err := store.Forget("tienda", "azul"); err != nil {
		t.Fatalf("Forget = %v, want no error", err)
	}

	registrada, _ := store.Worktree("tienda", "azul")
	if registrada != nil {
		t.Fatalf("registration after forgetting = %+v, want none", registrada)
	}
}

// ---------------------------------------------------------------- los slots

func TestElPrimerSlotEsElCero(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck

	reparto, err := store.Allocate("tienda", "azul")
	if err != nil {
		t.Fatalf("Allocate = %v, want no error", err)
	}

	if reparto.Slot != 0 || reparto.Schema != "m2_azul" {
		t.Fatalf("first allocation = %+v, want slot 0", reparto)
	}
}

func TestPedirDosVecesDaLoMismo(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck

	primero, _ := store.Allocate("tienda", "azul")
	segundo, err := store.Allocate("tienda", "azul")

	if err != nil || primero.Slot != segundo.Slot {
		t.Fatalf("asking twice = slots %d and %d (%v), want the same one", primero.Slot, segundo.Slot, err)
	}
}

func TestElSlotSeReutilizaCuandoLaRamaSeVa(t *testing.T) {
	// Sin esto, un proyecto se queda sin schemas que no está usando
	store := abierto(t)
	proyecto := core.Project{Name: "tienda"}

	for _, nombre := range []string{"azul", "verde", "roja"} {
		store.Register(proyecto, rama(nombre)) //nolint:errcheck
		store.Allocate("tienda", nombre)       //nolint:errcheck
	}

	if err := store.Forget("tienda", "verde"); err != nil {
		t.Fatalf("Forget = %v, want no error", err)
	}

	store.Register(proyecto, rama("nueva")) //nolint:errcheck

	reparto, err := store.Allocate("tienda", "nueva")
	if err != nil {
		t.Fatalf("Allocate = %v, want no error", err)
	}

	if reparto.Slot != 1 {
		t.Fatalf("slot after one was freed = %d, want the free one, 1", reparto.Slot)
	}
}

func TestOlvidarUnaRamaSeLlevaSuReparto(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck
	store.Allocate("tienda", "azul")                           //nolint:errcheck

	store.Forget("tienda", "azul") //nolint:errcheck

	reparto, err := store.Allocation("tienda", "azul")
	if err != nil {
		t.Fatalf("Allocation = %v, want no error", err)
	}

	if reparto != nil {
		t.Fatalf("allocation of a worktree that is gone = %+v, want none", reparto)
	}
}

func TestDosAgentesALaVezNoRecibenElMismoSlot(t *testing.T) {
	//
	// Esto es lo que la carpeta de JSON no daba, y no es una carrera rara: es lo que pasa la
	// primera vez que alguien lanza dos agentes en paralelo. El síntoma sería dos ramas
	// compartiendo base de datos.
	//
	store := abierto(t)
	proyecto := core.Project{Name: "tienda"}

	const cuantos = 12

	for i := 0; i < cuantos; i++ {
		store.Register(proyecto, rama(nombreDe(i))) //nolint:errcheck
	}

	repartos := make([]core.Allocation, cuantos)
	fallos := make([]error, cuantos)

	var todos sync.WaitGroup
	empezar := make(chan struct{})

	for i := 0; i < cuantos; i++ {
		todos.Add(1)

		go func(at int) {
			defer todos.Done()

			<-empezar

			repartos[at], fallos[at] = store.Allocate("tienda", nombreDe(at))
		}(i)
	}

	close(empezar)
	todos.Wait()

	vistos := map[int]string{}

	for i, reparto := range repartos {
		if fallos[i] != nil {
			t.Fatalf("allocation %d = %v, want no error", i, fallos[i])
		}

		if antes, repetido := vistos[reparto.Slot]; repetido {
			t.Fatalf("slot %d went to %q and to %q, want one each", reparto.Slot, antes, nombreDe(i))
		}

		vistos[reparto.Slot] = nombreDe(i)
	}

	if len(vistos) != cuantos {
		t.Fatalf("slots handed to twelve worktrees = %d, want 12", len(vistos))
	}
}

func nombreDe(at int) string { return string(rune('a'+at)) + "-rama" }

// ---------------------------------------------------------------- la anonimización

func TestLoQueNadieHaTocadoNoEstaAnonimizado(t *testing.T) {
	// Desconocido es la respuesta honesta, y nunca se trata como segura
	estado, cuando := abierto(t).Anonymisation("tienda")

	if estado != "unknown" || cuando != "" {
		t.Fatalf("anonymisation with nothing recorded = %q, %q, want unknown", estado, cuando)
	}
}

func TestSeApuntaYSeBorra(t *testing.T) {
	store := abierto(t)

	store.RecordAnonymisation("tienda", "2026-09-02 12:00") //nolint:errcheck

	if estado, cuando := store.Anonymisation("tienda"); estado != "yes" || cuando == "" {
		t.Fatalf("anonymisation = %q, %q, want what was recorded", estado, cuando)
	}

	// Todo lo que reemplaza la base de datos lo borra: un "sí" heredado de antes de una
	// importación es peor que no tener registro
	store.ClearAnonymisation("tienda") //nolint:errcheck

	if estado, _ := store.Anonymisation("tienda"); estado != "unknown" {
		t.Fatalf("anonymisation after the data was replaced = %q, want unknown", estado)
	}
}

func TestCadaEntornoResponsePorSusDatos(t *testing.T) {
	// Una rama tiene su propia base de datos: responder por ella con el registro del principal es
	// justo el error que importa
	store := abierto(t)
	store.RecordAnonymisation("tienda", "2026-09-02 12:00") //nolint:errcheck

	if estado, _ := store.Anonymisation("tienda-azul"); estado != "unknown" {
		t.Fatalf("anonymisation of a branch = %q, want it not inherited from the main one", estado)
	}
}

// ---------------------------------------------------------------- lo que ya había en disco

func TestTraerseLoQueEscribioBash(t *testing.T) {
	casa := t.TempDir()
	worktrees := filepath.Join(casa, "worktrees", "tienda")
	estado := filepath.Join(casa, "state")

	os.MkdirAll(worktrees, 0o755) //nolint:errcheck
	os.MkdirAll(estado, 0o755)    //nolint:errcheck

	os.WriteFile(filepath.Join(worktrees, "azul.json"), []byte(`{
		"path": "/code/tienda-azul", "branch": "feature/azul", "profile": "agent",
		"domain": "azul.tienda.test", "project": "tienda-azul",
		"created": "2026-08-01 10:00", "vendor": "shared"}`), 0o644) //nolint:errcheck

	os.WriteFile(filepath.Join(estado, "tienda.json"),
		[]byte(`{"anonymised_at": "2026-08-02 11:00"}`), 0o644) //nolint:errcheck

	store := abierto(t)

	traido, err := store.Import(filepath.Join(casa, "worktrees"), estado)
	if err != nil {
		t.Fatalf("Import = %v, want no error", err)
	}

	if traido.Worktrees != 1 || traido.States != 1 {
		t.Fatalf("imported = %+v, want one worktree and one state", traido)
	}

	registrada, _ := store.Worktree("tienda", "azul")
	if registrada == nil || !registrada.SharedVendor || registrada.Domain != "azul.tienda.test" {
		t.Fatalf("registration = %+v, want everything it recorded", registrada)
	}

	if cual, _ := store.Anonymisation("tienda"); cual != "yes" {
		t.Fatalf("anonymisation = %q, want it imported too", cual)
	}
}

func TestImportarDosVecesNoDuplica(t *testing.T) {
	// Corre en máquinas con gente trabajando dentro, así que tiene que poder repetirse
	casa := t.TempDir()
	worktrees := filepath.Join(casa, "worktrees", "tienda")
	os.MkdirAll(worktrees, 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(worktrees, "azul.json"),
		[]byte(`{"path": "/code/a", "project": "tienda-azul"}`), 0o644) //nolint:errcheck

	store := abierto(t)
	store.Import(filepath.Join(casa, "worktrees"), filepath.Join(casa, "state")) //nolint:errcheck
	store.Import(filepath.Join(casa, "worktrees"), filepath.Join(casa, "state")) //nolint:errcheck

	ramas, _ := store.Worktrees("tienda")
	if len(ramas) != 1 {
		t.Fatalf("worktrees after importing twice = %+v, want one", ramas)
	}
}

func TestUnRegistroIlegibleNoSeLlevaALosDemas(t *testing.T) {
	casa := t.TempDir()
	worktrees := filepath.Join(casa, "worktrees", "tienda")
	os.MkdirAll(worktrees, 0o755)                                                          //nolint:errcheck
	os.WriteFile(filepath.Join(worktrees, "rota.json"), []byte("{esto no es json"), 0o644) //nolint:errcheck
	os.WriteFile(filepath.Join(worktrees, "buena.json"),
		[]byte(`{"path": "/code/b", "project": "tienda-buena"}`), 0o644) //nolint:errcheck

	store := abierto(t)

	traido, _ := store.Import(filepath.Join(casa, "worktrees"), filepath.Join(casa, "state"))

	if traido.Worktrees != 1 {
		t.Fatalf("imported with one unreadable = %+v, want the others brought in", traido)
	}
}

// ---------------------------------------------------------------- el fichero

func TestAbrirDosVecesNoRompeNada(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "hm.db")

	primero, err := Open(ruta)
	if err != nil {
		t.Fatalf("opening it = %v, want no error", err)
	}

	primero.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck
	primero.Close()                                              //nolint:errcheck

	segundo, err := Open(ruta)
	if err != nil {
		t.Fatalf("opening it again = %v, want no error", err)
	}
	defer segundo.Close()

	registrada, _ := segundo.Worktree("tienda", "azul")
	if registrada == nil {
		t.Fatal("what was written before = gone, want it still there")
	}
}
