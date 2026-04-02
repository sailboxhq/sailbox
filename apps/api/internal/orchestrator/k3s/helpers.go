package k3s

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

func intOrString(val int) *intstr.IntOrString {
	v := intstr.FromInt32(int32(val))
	return &v
}

func int32Ptr(i int32) *int32 {
	return &i
}
