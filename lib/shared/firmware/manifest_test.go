package firmware

import "testing"

func TestManifestValidate(t *testing.T) {
	manifest := Manifest{
		Package: "github.com/burgrp/hw-fan-switch/fw",
		TinyGo: TinyGoProfile{
			Scheduler:        SchedulerTasks,
			StackSizeBytes:   1024,
			GarbageCollector: GCLeaking,
			Serial:           SerialRTT,
			SizeReport:       SizeReportHTML,
			PrintAllocs:      true,
		},
		PyOCD: PyOCDProfile{
			Reclaim:                  true,
			ReclaimDelayMilliseconds: 1000,
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestManifestValidateRejectsCommandFragments(t *testing.T) {
	manifest := Manifest{Package: "github.com/burgrp/fw;touch"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected invalid package error")
	}
}

func TestManifestValidateRejectsDelayWithoutReclaim(t *testing.T) {
	manifest := Manifest{
		Package: "github.com/burgrp/hw-fan-switch/fw",
		PyOCD: PyOCDProfile{
			ReclaimDelayMilliseconds: 1000,
		},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected reclaim delay error")
	}
}
