package builtin

import "testing"

func TestHasAllFlag(t *testing.T) {
	yes := [][]string{{"-all"}, {"--all"}, {"-a"}, {"-ALL"}, {"weather", "-all"}}
	for _, a := range yes {
		if !hasAllFlag(a) {
			t.Errorf("%v: want true", a)
		}
	}
	no := [][]string{{"all"}, {"weather"}, {"-alls"}, nil}
	for _, a := range no {
		if hasAllFlag(a) {
			t.Errorf("%v: want false", a)
		}
	}
}

func TestAptSubcommandsKeepAll(t *testing.T) {
	if aptSubcommands["i"] != "install" || aptSubcommands["rm"] != "remove" {
		t.Fatal("short subcommands changed")
	}
}
