// Command two_stage_mapping_complexity_sched runs the SWE-bench Lite
// two-stage mapping experiment with complexity-aware heavy-queue scheduling.
package main

import (
	"context"
	"log"
	"os"

	"github.com/dorcha-inc/orla/examples/swe_bench_lite/shared"
	twostagemappingcomplexitysched "github.com/dorcha-inc/orla/examples/swe_bench_lite/two_stage_mapping_complexity_sched"
)

func main() {
	log.Println("================================================")
	log.Println("Running two-stage mapping complexity scheduling experiment")
	log.Println("================================================")

	ctx := context.Background()
	dataset, err := shared.LoadDataset()
	if err != nil {
		log.Fatal(err)
	}
	if err := twostagemappingcomplexitysched.Run(ctx, dataset); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
