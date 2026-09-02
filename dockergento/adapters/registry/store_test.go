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
		t.Fatalf("no se pudo abrir el registro: %v", err)
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
		t.Fatalf("preguntar por lo que no hay no es un fallo: %v", err)
	}

	if registrada != nil {
		t.Fatalf("no debería haber nada: %+v", registrada)
	}
}

func TestRegistrarYLeerUnaRama(t *testing.T) {
	store := abierto(t)

	if err := store.Register(core.Project{Name: "tienda", Root: "/code/tienda"}, rama("azul")); err != nil {
		t.Fatalf("registro fallido: %v", err)
	}

	registrada, err := store.Worktree("tienda", "azul")
	if err != nil || registrada == nil {
		t.Fatalf("no se recuperó: %v", err)
	}

	if registrada.Project != "tienda-azul" || registrada.Domain != "azul.tienda.test" {
		t.Fatalf("el nombre y la dirección salen del registro, no se derivan: %+v", registrada)
	}

	if registrada.Parent != "tienda" {
		t.Fatalf("tiene que saber de quién es hija: %q", registrada.Parent)
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
		t.Fatalf("segundo registro fallido: %v", err)
	}

	ramas, err := store.Worktrees("tienda")
	if err != nil || len(ramas) != 1 {
		t.Fatalf("una rama, no dos: %v %+v", err, ramas)
	}

	if ramas[0].Branch != "otra-rama" || !ramas[0].SharedVendor {
		t.Fatalf("tenía que quedarse con lo último: %+v", ramas[0])
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
		t.Fatal("el registro tenía que rechazarlo")
	}
}

func TestOlvidarUnaRama(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck

	if err := store.Forget("tienda", "azul"); err != nil {
		t.Fatalf("no se pudo olvidar: %v", err)
	}

	registrada, _ := store.Worktree("tienda", "azul")
	if registrada != nil {
		t.Fatalf("seguía ahí: %+v", registrada)
	}
}

// ---------------------------------------------------------------- los slots

func TestElPrimerSlotEsElCero(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck

	reparto, err := store.Allocate("tienda", "azul")
	if err != nil {
		t.Fatalf("reparto fallido: %v", err)
	}

	if reparto.Slot != 0 || reparto.Schema != "m2_azul" {
		t.Fatalf("el primero es el cero: %+v", reparto)
	}
}

func TestPedirDosVecesDaLoMismo(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck

	primero, _ := store.Allocate("tienda", "azul")
	segundo, err := store.Allocate("tienda", "azul")

	if err != nil || primero.Slot != segundo.Slot {
		t.Fatalf("un reparto es de quien lo tiene: %v %d %d", err, primero.Slot, segundo.Slot)
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
		t.Fatalf("no se pudo olvidar: %v", err)
	}

	store.Register(proyecto, rama("nueva")) //nolint:errcheck

	reparto, err := store.Allocate("tienda", "nueva")
	if err != nil {
		t.Fatalf("reparto fallido: %v", err)
	}

	if reparto.Slot != 1 {
		t.Fatalf("el hueco que dejó 'verde' era el 1, y dio el %d", reparto.Slot)
	}
}

func TestOlvidarUnaRamaSeLlevaSuReparto(t *testing.T) {
	store := abierto(t)
	store.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck
	store.Allocate("tienda", "azul")                           //nolint:errcheck

	store.Forget("tienda", "azul") //nolint:errcheck

	reparto, err := store.Allocation("tienda", "azul")
	if err != nil {
		t.Fatalf("consulta fallida: %v", err)
	}

	if reparto != nil {
		t.Fatalf("una rama que ya no está no puede tener schema: %+v", reparto)
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
			t.Fatalf("reparto %d fallido: %v", i, fallos[i])
		}

		if antes, repetido := vistos[reparto.Slot]; repetido {
			t.Fatalf("el slot %d se dio a %q y a %q", reparto.Slot, antes, nombreDe(i))
		}

		vistos[reparto.Slot] = nombreDe(i)
	}

	if len(vistos) != cuantos {
		t.Fatalf("doce ramas, doce slots, y salieron %d", len(vistos))
	}
}

func nombreDe(at int) string { return string(rune('a'+at)) + "-rama" }

// ---------------------------------------------------------------- la anonimización

func TestLoQueNadieHaTocadoNoEstaAnonimizado(t *testing.T) {
	// Desconocido es la respuesta honesta, y nunca se trata como segura
	estado, cuando := abierto(t).Anonymisation("tienda")

	if estado != "unknown" || cuando != "" {
		t.Fatalf("sin registro no hay garantía: %q %q", estado, cuando)
	}
}

func TestSeApuntaYSeBorra(t *testing.T) {
	store := abierto(t)

	store.RecordAnonymisation("tienda", "2026-09-02 12:00") //nolint:errcheck

	if estado, cuando := store.Anonymisation("tienda"); estado != "yes" || cuando == "" {
		t.Fatalf("tenía que estar apuntado: %q %q", estado, cuando)
	}

	// Todo lo que reemplaza la base de datos lo borra: un "sí" heredado de antes de una
	// importación es peor que no tener registro
	store.ClearAnonymisation("tienda") //nolint:errcheck

	if estado, _ := store.Anonymisation("tienda"); estado != "unknown" {
		t.Fatalf("después de reemplazar los datos no hay garantía: %q", estado)
	}
}

func TestCadaEntornoResponsePorSusDatos(t *testing.T) {
	// Una rama tiene su propia base de datos: responder por ella con el registro del principal es
	// justo el error que importa
	store := abierto(t)
	store.RecordAnonymisation("tienda", "2026-09-02 12:00") //nolint:errcheck

	if estado, _ := store.Anonymisation("tienda-azul"); estado != "unknown" {
		t.Fatalf("la rama no hereda la garantía del principal: %q", estado)
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
		t.Fatalf("importación fallida: %v", err)
	}

	if traido.Worktrees != 1 || traido.States != 1 {
		t.Fatalf("una rama y un estado: %+v", traido)
	}

	registrada, _ := store.Worktree("tienda", "azul")
	if registrada == nil || !registrada.SharedVendor || registrada.Domain != "azul.tienda.test" {
		t.Fatalf("no llegó entera: %+v", registrada)
	}

	if cual, _ := store.Anonymisation("tienda"); cual != "yes" {
		t.Fatalf("el estado de los datos también viene: %q", cual)
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
		t.Fatalf("dos importaciones, una rama: %+v", ramas)
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
		t.Fatalf("una ilegible no es motivo para dejar atrás las otras: %+v", traido)
	}
}

// ---------------------------------------------------------------- el fichero

func TestAbrirDosVecesNoRompeNada(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "hm.db")

	primero, err := Open(ruta)
	if err != nil {
		t.Fatalf("primera apertura fallida: %v", err)
	}

	primero.Register(core.Project{Name: "tienda"}, rama("azul")) //nolint:errcheck
	primero.Close()                                              //nolint:errcheck

	segundo, err := Open(ruta)
	if err != nil {
		t.Fatalf("segunda apertura fallida: %v", err)
	}
	defer segundo.Close()

	registrada, _ := segundo.Worktree("tienda", "azul")
	if registrada == nil {
		t.Fatal("lo escrito antes tenía que seguir ahí")
	}
}
