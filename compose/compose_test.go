//go:build !windows

package compose

//func TestNewProject(t *testing.T) {
//	type args struct {
//		name string
//		conf *models.Config
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    *types.Project
//		wantErr bool
//	}{
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := NewADCMProject(tt.args.name, tt.args.conf)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("NewADCMProject() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("NewADCMProject() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

//func TestNewService(t *testing.T) {
//	type args struct {
//		projectName string
//		name        string
//		image       string
//	}
//	tests := []struct {
//		name string
//		args args
//		want types.ServiceConfig
//	}{
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := NewService(tt.args.projectName, tt.args.name, tt.args.image); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("NewService() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
