package core

import "testing"

// The derivation is pure so that the naming can be checked without a database, and derived rather
// than stored field by field so that two worktrees cannot end up sharing a schema through a bad
// write.

func TestCadaRamaTieneSuPropioSchema(t *testing.T) {
	vistos := map[string]string{}

	for _, nombre := range []string{"azul", "verde", "feature-x", "hotfix-1"} {
		reparto := AllocationFor(nombre, 0)

		if antes, repetido := vistos[reparto.Schema]; repetido {
			t.Fatalf("%q y %q comparten el schema %q", antes, nombre, reparto.Schema)
		}

		vistos[reparto.Schema] = nombre
	}
}

func TestLasTresBasesDeRedisNoSePisan(t *testing.T) {
	// Magento separa caché, caché de página y sesiones: mezclarlas entre ramas es cómo un flush
	// en una tira las sesiones de otra
	usadas := map[int]int{}

	for slot := 0; slot < MaxSlots; slot++ {
		reparto := AllocationFor("rama", slot)

		for _, db := range []int{reparto.CacheDB, reparto.PageCacheDB, reparto.SessionDB} {
			if antes, repetida := usadas[db]; repetida {
				t.Fatalf("la base %d es del slot %d y también del %d", db, antes, slot)
			}

			usadas[db] = slot
		}
	}

	if len(usadas) != MaxSlots*3 {
		t.Fatalf("tres bases por slot: %d", len(usadas))
	}
}

func TestNoSeSalenDeLoQueRedisTiene(t *testing.T) {
	ultimo := AllocationFor("rama", MaxSlots-1)

	if ultimo.SessionDB >= RedisDatabases {
		t.Fatalf("la base %d no existe en un Redis con %d", ultimo.SessionDB, RedisDatabases)
	}
}

func TestElSchemaEsUnNombreQueMariaDBAcepta(t *testing.T) {
	// Un nombre de rama lleva guiones y un schema no puede
	reparto := AllocationFor("feature-algo-largo", 0)

	if reparto.Schema != "m2_feature_algo_largo" {
		t.Fatalf("los guiones no valen en un schema: %q", reparto.Schema)
	}
}

func TestQuedarseSinSlotsSeExplica(t *testing.T) {
	err := ErrNoSlots{Project: "tienda"}

	if err.Error() == "" {
		t.Fatal("una negativa sin motivo no sirve de nada")
	}
}
