package config

import "testing"

func TestParseConfigBytesDoesNotMutateGlobalOptions(t *testing.T) {
	previous := Opt
	defer func() { Opt = previous }()
	Opt.Advanced.Dir = "global-sentinel"
	input := []byte(`
[scan_reader]
address = "127.0.0.1:6379"
scan = true

[redis_writer]
address = "127.0.0.1:6380"

[advanced]
dir = "generated"
`)
	v, options, err := ParseConfigBytes(input)
	if err != nil {
		t.Fatalf("ParseConfigBytes() error = %v", err)
	}
	if err := ValidateConfigSections(v); err != nil {
		t.Fatalf("ValidateConfigSections() error = %v", err)
	}
	if options.Advanced.Dir != "generated" {
		t.Fatalf("parsed advanced.dir = %q", options.Advanced.Dir)
	}
	if Opt.Advanced.Dir != "global-sentinel" {
		t.Fatalf("ParseConfigBytes() mutated global Opt.Advanced.Dir = %q", Opt.Advanced.Dir)
	}
}

func TestValidateConfigSections(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name: "one reader and writer",
			config: `[sync_reader]
address = "127.0.0.1:6379"
[redis_writer]
address = "127.0.0.1:6380"`,
		},
		{
			name: "multiple readers",
			config: `[sync_reader]
address = "127.0.0.1:6379"
[scan_reader]
address = "127.0.0.1:6379"
[redis_writer]
address = "127.0.0.1:6380"`,
			wantErr: true,
		},
		{
			name: "missing writer",
			config: `[scan_reader]
address = "127.0.0.1:6379"`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			v, _, err := ParseConfigBytes([]byte(test.config))
			if err != nil {
				t.Fatalf("ParseConfigBytes() error = %v", err)
			}
			err = ValidateConfigSections(v)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateConfigSections() error = %v, wantErr = %v", err, test.wantErr)
			}
		})
	}
}
