package models

import (
	"fmt"
	"github.com/arenadata/adcm-installer/utils"
	"testing"
)

func TestImage_String(t *testing.T) {
	defaultRegistry := utils.Ptr(ADImageRegistry)
	defaultImage := utils.Ptr(ADCMImageName)
	defaultTag := utils.Ptr(ADCMImageTag)
	hashedTag := utils.Ptr("sha256:252e755d271823d66e43026077ced693104105d0e91f5c3fa4a23159d628b007")

	type fields struct {
		Registry *string
		Name     *string
		Tag      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			"WithoutRegistryWithoutTag",
			fields{nil, defaultImage, nil},
			fmt.Sprintf("%s:%s", ADCMImageName, DefaultImageTag),
		},
		{
			"WithoutRegistry",
			fields{nil, defaultImage, defaultTag},
			fmt.Sprintf("%s:%s", ADCMImageName, ADCMImageTag),
		},
		{
			"FullImage",
			fields{defaultRegistry, defaultImage, defaultTag},
			fmt.Sprintf("%s/%s:%s", ADImageRegistry, ADCMImageName, ADCMImageTag),
		},
		{
			"FullImageWithHashedTag",
			fields{defaultRegistry, defaultImage, hashedTag},
			fmt.Sprintf("%s/%s@%s", ADImageRegistry, ADCMImageName, *hashedTag),
		},
		{
			"NonClearNames-1",
			fields{utils.Ptr(ADImageRegistry + "/"), defaultImage, defaultTag},
			fmt.Sprintf("%s/%s:%s", ADImageRegistry, ADCMImageName, ADCMImageTag),
		},
		{
			"NonClearNames-2",
			fields{utils.Ptr(ADImageRegistry + "/"), utils.Ptr("/" + ADCMImageName + "/"), defaultTag},
			fmt.Sprintf("%s/%s:%s", ADImageRegistry, ADCMImageName, ADCMImageTag),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Image{
				Registry: tt.fields.Registry,
				Name:     tt.fields.Name,
				Tag:      tt.fields.Tag,
			}
			if got := i.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
