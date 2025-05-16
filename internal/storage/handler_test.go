package storage

import (
	"context"
	"golang.hedera.com/solo-cheetah/internal/core"
	"testing"
)

func Test_handler_computeDestinationPath(t *testing.T) {
	type fields struct {
		id             string
		storageType    string
		fileExtensions []string
		rootDir        string
		pathPrefix     string
		preSync        func(ctx context.Context) error
		syncFile       func(ctx context.Context, src string, dest string) (*core.UploadInfo, error)
	}
	type args struct {
		srcDir   string
		fileName string
		ext      string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "Test with sub directory",
			fields: fields{
				id:             "test-handler",
				storageType:    "local",
				fileExtensions: []string{".txt", ".jpg"},
				rootDir:        "/root",
				pathPrefix:     "uploads",
			},
			args: args{
				srcDir:   "/root/recordStream/record0.0.10",
				fileName: "file",
				ext:      ".txt",
			},
			want: "uploads/recordStream/record0.0.10/file.txt",
		},
		{
			name: "Test without sub directory",
			fields: fields{
				id:             "test-handler",
				storageType:    "local",
				fileExtensions: []string{".txt", ".jpg"},
				rootDir:        "/root",
				pathPrefix:     "uploads",
			},
			args: args{
				srcDir:   "/root",
				fileName: "file",
				ext:      ".txt",
			},
			want: "uploads/file.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				id:             tt.fields.id,
				storageType:    tt.fields.storageType,
				fileExtensions: tt.fields.fileExtensions,
				rootDir:        tt.fields.rootDir,
				pathPrefix:     tt.fields.pathPrefix,
				preSync:        tt.fields.preSync,
				syncFile:       tt.fields.syncFile,
			}
			if got := h.computeDestinationPath(tt.args.srcDir, tt.args.fileName, tt.args.ext); got != tt.want {
				t.Errorf("computeDestinationPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
