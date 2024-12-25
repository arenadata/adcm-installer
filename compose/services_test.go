package compose

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/arenadata/adcm-installer/models"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestVolume(t *testing.T) {
	bindVolume := func(path string) types.ServiceVolumeConfig {
		return types.ServiceVolumeConfig{
			Type:   types.VolumeTypeBind,
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			Source: path,
			Target: models.ADCMVolumeTarget,
		}
	}

	namedVolume := types.ServiceVolumeConfig{
		Type:   types.VolumeTypeVolume,
		Volume: &types.ServiceVolumeVolume{},
		Source: models.ADCMVolumeName,
		Target: models.ADCMVolumeTarget,
	}

	type args struct {
		volume    string
		defSrc    string
		defTarget string
	}
	tests := []struct {
		name    string
		args    args
		want    types.ServiceVolumeConfig
		wantErr bool
	}{
		{
			name:    "EmptyVolumeWithEmptyDefaults",
			args:    args{},
			want:    types.ServiceVolumeConfig{},
			wantErr: true,
		},
		{
			name: "EmptyVolumeWithEmptyDefaultTarget",
			args: args{
				defSrc: models.ADCMVolumeName,
			},
			want:    types.ServiceVolumeConfig{},
			wantErr: true,
		},
		{
			name: "EmptyVolumeWithDefaults",
			args: args{
				defSrc:    models.ADCMVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithoutSource",
			args: args{
				volume:    ":" + models.ADCMVolumeTarget,
				defSrc:    models.ADCMVolumeName,
				defTarget: models.PostgresVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithoutTargetWithEmptyDefaultTarget",
			args: args{
				volume: models.ADCMVolumeName,
				defSrc: models.ADCMVolumeName,
			},
			want:    types.ServiceVolumeConfig{},
			wantErr: true,
		},
		{
			name: "NamedVolumeWithoutTargetWithEmptyDefaultSource",
			args: args{
				volume:    models.ADCMVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithEmptyTargetWithEmptyDefaultSource",
			args: args{
				volume:    models.ADCMVolumeName + ":",
				defTarget: models.ADCMVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithTargetWithEmptyDefaults",
			args: args{
				volume: models.ADCMVolumeName + ":" + models.ADCMVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithoutTargetWithDefaults",
			args: args{
				volume:    models.ADCMVolumeName,
				defSrc:    models.PostgresVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithEmptyTargetWithDefaults",
			args: args{
				volume:    models.ADCMVolumeName + ":",
				defSrc:    models.PostgresVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "NamedVolumeWithTargetWithDefaults",
			args: args{
				volume:    models.ADCMVolumeName + ":" + models.ADCMVolumeTarget,
				defSrc:    models.PostgresVolumeName,
				defTarget: models.PostgresVolumeTarget,
			},
			want: namedVolume,
		},
		{
			name: "WithEmptySourceWithDefaultTargetRelativePathCurrentDirectory",
			args: args{
				volume: ":" + models.ADCMVolumeTarget,
				defSrc: ".",
			},
			want: bindVolume(absPath(".")),
		},
		{
			name: "RelativePathCurrentDirectoryWithoutTargetWithEmptyDefaultTarget",
			args: args{
				volume: ".",
				defSrc: models.ADCMVolumeName,
			},
			want:    types.ServiceVolumeConfig{},
			wantErr: true,
		},
		{
			name: "RelativePathCurrentDirectoryWithEmptyTargetWithEmptyDefaultTarget",
			args: args{
				volume: ".:",
				defSrc: models.ADCMVolumeName,
			},
			want:    types.ServiceVolumeConfig{},
			wantErr: true,
		},
		{
			name: "WindowsPath",
			args: args{
				volume: "C:\\data:" + models.ADCMVolumeTarget,
			},
			want:    types.ServiceVolumeConfig{},
			wantErr: true,
		},
		{
			name: "RelativePathCurrentDirectoryWithEmptyTarget",
			args: args{
				volume:    ".",
				defSrc:    models.ADCMVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: bindVolume(absPath(".")),
		},
		{
			name: "RelativePathCurrentDirectoryWithEmptyTarget",
			args: args{
				volume:    ".:",
				defSrc:    models.ADCMVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: bindVolume(absPath(".")),
		},
		{
			name: "RelativePathCurrentDirectoryWithTarget",
			args: args{
				volume:    ".:" + models.ADCMVolumeTarget,
				defSrc:    models.ADCMVolumeName,
				defTarget: models.PostgresVolumeName,
			},
			want: bindVolume(absPath(".")),
		},
		{
			name: "AbsolutePathSpecificDirectoryWithEmptyTarget",
			args: args{
				volume:    "/path/to/something",
				defSrc:    models.ADCMVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: bindVolume("/path/to/something"),
		},
		{
			name: "AbsolutePathSpecificDirectoryWithEmptyTarget",
			args: args{
				volume:    "/path/to/something:",
				defSrc:    models.ADCMVolumeName,
				defTarget: models.ADCMVolumeTarget,
			},
			want: bindVolume("/path/to/something"),
		},
		{
			name: "AbsolutePathSpecificDirectoryWithTarget",
			args: args{
				volume:    "/path/to/something:" + models.ADCMVolumeTarget,
				defSrc:    models.ADCMVolumeName,
				defTarget: models.PostgresVolumeTarget,
			},
			want: bindVolume("/path/to/something"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Volume(tt.args.volume, tt.args.defSrc, tt.args.defTarget)
			if (err != nil) != tt.wantErr {
				t.Errorf("Volume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Volume() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func absPath(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}

	return p
}
