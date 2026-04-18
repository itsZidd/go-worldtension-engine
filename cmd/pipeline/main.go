package main

import (
	"fmt"
	"log"
	"time"

	"go-worldtension-engine/internal/engine/aggregator"
	"go-worldtension-engine/internal/platform/gdelt"
	"go-worldtension-engine/internal/platform/worldbank"
)

func main() {
	fmt.Println("🌍 Starting WorldTension Engine Pipeline...")
	fmt.Println("=========================================")

	testCountry := "USA"
	compareRegion := "HIC"

	// ---------------------------------------------------
	// 1. Fetch USA Anchor
	// ---------------------------------------------------
	fmt.Printf("\n⚓ Fetching industrial baseline for %s...\n", testCountry)

	start := time.Now()

	usaAnchorScore, err := worldbank.FetchIndustrialBaseline(testCountry)
	if err != nil {
		log.Printf("⚠️ Failed to fetch %s baseline: %v\n", testCountry, err)
		usaAnchorScore = 50.0 // fallback score
		fmt.Printf("🛟 Using fallback baseline for %s: %.2f / 100\n", testCountry, usaAnchorScore)
	} else {
		fmt.Printf("✅ %s Industrial Base: %.2f / 100\n", testCountry, usaAnchorScore)
	}

	fmt.Printf("⏱️ Took %s\n", time.Since(start))

	// small cooldown (optional)
	time.Sleep(2 * time.Second)

	// ---------------------------------------------------
	// 2. Fetch Region Average
	// ---------------------------------------------------
	fmt.Printf("\n🌐 Fetching regional benchmark: %s...\n", compareRegion)

	start = time.Now()

	regionAnchorScore, err := worldbank.FetchIndustrialBaseline(compareRegion)
	if err != nil {
		log.Printf("⚠️ Failed to fetch %s benchmark: %v\n", compareRegion, err)
	} else {
		fmt.Printf("✅ %s Industrial Base: %.2f / 100\n", compareRegion, regionAnchorScore)
	}

	fmt.Printf("⏱️ Took %s\n", time.Since(start))

	// ---------------------------------------------------
	// 3. Prepare Anchors for Aggregator
	// ---------------------------------------------------
	anchors := map[string]float64{
		testCountry: usaAnchorScore,
	}

	// ---------------------------------------------------
	// 4. Fetch Latest GDELT Signals
	// ---------------------------------------------------
	fmt.Println("\n📰 Fetching latest GDELT signals...")

	start = time.Now()

	signals, err := gdelt.FetchLatest()
	if err != nil {
		log.Fatalf("❌ Failed to fetch GDELT data: %v", err)
	}

	fmt.Printf("✅ GDELT Quality Gate Passed (%d signals)\n", len(signals))
	fmt.Printf("⏱️ Took %s\n", time.Since(start))

	// ---------------------------------------------------
	// 5. Run Aggregator
	// ---------------------------------------------------
	fmt.Println("\n⚙️ Running tension aggregation engine...")

	start = time.Now()

	snapshots := aggregator.Process(signals, anchors)

	fmt.Printf("✅ Aggregation complete in %s\n", time.Since(start))

	// ---------------------------------------------------
	// 6. Output Final Snapshot
	// ---------------------------------------------------
	if snapshot, ok := snapshots[testCountry]; ok {
		fmt.Println("\n=========================================")
		fmt.Printf("🌐 FINAL GAME STATE: %s\n", testCountry)
		fmt.Println("=========================================")

		fmt.Printf("Total Events Tracked : %d\n", snapshot.EventCount)
		fmt.Printf("Current Tension      : %.2f / 100\n", snapshot.TensionScore)
		fmt.Printf("Industrial Capacity  : %.2f / 100\n", snapshot.IndustrialCapacity)

		if regionAnchorScore > 0 {
			diff := usaAnchorScore - regionAnchorScore
			fmt.Printf("Vs %s Benchmark      : %+0.2f\n", compareRegion, diff)
		}
	} else {
		fmt.Printf("⚠️ No snapshot generated for %s\n", testCountry)
	}

	fmt.Println("\n🏁 Pipeline completed successfully.")
}
