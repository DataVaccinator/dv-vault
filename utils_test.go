package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestGetCurrentDateTime(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"check format", string(time.Now().Format("2006-01-02 15:04:05"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCurrentDateTime(); got != tt.want {
				t.Errorf("GetCurrentDateTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	type args struct {
		clientRequest interface{}
		asDefault     string
	}
	var manyType interface{} = "test"
	var stringType string = "test"
	var floatType float64 = 123
	var invalidType interface{} = nil

	tests := []struct {
		name    string
		args    args
		wantRes string
	}{
		{"with interface", args{manyType, "default"}, "test"},
		{"with string", args{stringType, "default"}, "test"},
		{"with float", args{floatType, "default"}, "123"},
		{"with empty", args{invalidType, "default"}, "default"},
		{"with nil", args{nil, "default"}, "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := GetString(tt.args.clientRequest, tt.args.asDefault); gotRes != tt.wantRes {
				t.Errorf("GetString() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	type args struct {
		clientRequest interface{}
		asDefault     int
	}
	var manyType interface{} = 123
	var stringType string = "123"
	var floatType float64 = 123.0001
	var invalidType interface{} = nil

	tests := []struct {
		name    string
		args    args
		wantRes int
	}{
		{"with interface", args{manyType, 1}, 123},
		{"with string", args{stringType, 1}, 123},
		{"with float", args{floatType, 1}, 123},
		{"with empty", args{invalidType, 1}, 1},
		{"with nil", args{nil, 1}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := GetInt(tt.args.clientRequest, tt.args.asDefault); gotRes != tt.wantRes {
				t.Errorf("GetInt() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func TestGetStringArray(t *testing.T) {
	type args struct {
		clientRequest interface{}
		asDefault     []string
	}
	var manyType = []interface{}{"first", "second", 1234, 123.45}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{"with valid interface array", args{manyType, []string{}}, []string{"first", "second", "1234", "123.45"}},
		{"with nil", args{nil, []string{"default"}}, []string{"default"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetStringArray(tt.args.clientRequest, tt.args.asDefault); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStringArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateVID(t *testing.T) {
	t.Run("check length", func(t *testing.T) {
		if got := GenerateVID(); len(got) != 32 {
			fmt.Println(len(got))
			t.Errorf("GenerateVID() = %v, want %v", got, "length of 32 chars")
		}
	})
}

func TestValidateVID(t *testing.T) {
	t.Run("valid one", func(t *testing.T) {
		if got := ValidateVID("8f494a220b95ec7440dcc9f453709dc0"); got != true {
			t.Errorf("ValidateVID() = %v, want %v", got, true)
		}
	})
	t.Run("invalid one", func(t *testing.T) {
		if got := ValidateVID("fdsfsöä5ec7440dcc9f453709dc0"); got != false {
			t.Errorf("ValidateVID() = %v, want %v", got, false)
		}
	})
}

func TestMakeUnique(t *testing.T) {
	t.Run("lets reduce", func(t *testing.T) {
		if got := MakeUnique([]string{"a", "b", "b", "a"}); len(got) != 2 {
			t.Errorf("MakeUnique() = %v, want %v entries", len(got), 2)
		}
	})
}

func TestValidateSearchWord(t *testing.T) {
	t.Run("valid one", func(t *testing.T) {
		if got := ValidateSearchWord("8f494a220b95"); got != true {
			t.Errorf("ValidateSearchWord() = %v, want %v", got, true)
		}
	})
	t.Run("invalid one", func(t *testing.T) {
		if got := ValidateSearchWord("8f494a2 20b95"); got != false {
			t.Errorf("ValidateSearchWord() = %v, want %v", got, false)
		}
	})
}
