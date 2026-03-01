// Command two_stage_mapping runs the SWE-bench Lite two-stage mapping experiment.
package main

import (
	"context"
	"log"
	"os"

	"github.com/dorcha-inc/orla/examples/swe_bench_lite/shared"
	twostagemapping "github.com/dorcha-inc/orla/examples/swe_bench_lite/two_stage_mapping"
)

func main() {

	log.Println("================================================")
	log.Println("Running two-stage mapping experiment")
	log.Println("================================================")

	ctx := context.Background()
	dataset, err := shared.LoadDataset()
	if err != nil {
		log.Fatal(err)
	}
	if err := twostagemapping.Run(ctx, dataset); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
