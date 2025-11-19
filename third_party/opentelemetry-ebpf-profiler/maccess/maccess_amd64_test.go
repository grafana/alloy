//go:build amd64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maccess

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:lll
var codeblobs = map[string]struct {
	code                    []byte
	copyFromUserNofaultAddr uint64
	nmiUaccessOkayAddr      uint64
	isPatched               bool
}{
	"Debian - 6.1.0-13-amd64": {
		isPatched:               true,
		copyFromUserNofaultAddr: 18446744072414051808, // 0xffffffff81283de0
		nmiUaccessOkayAddr:      18446744072411965904, // 0xffffffff810869d0
		code: []byte{
			0xe8, 0x1b, 0xd4, 0xde, 0xff, //             call   ffffffff81071200 <__fentry__>
			0x48, 0xb8, 0x00, 0xf0, 0xff, 0xff, 0xff, // movabs $0x7ffffffff000,%rax
			0x7f, 0x00, 0x00,
			0x48, 0x39, 0xd0, //                         cmp    %rdx,%rax
			0x73, 0x0c, //                               jae    ffffffff81283e00 <copy_from_user_nofault+0x20>
			0x48, 0xc7, 0xc0, 0xf2, 0xff, 0xff, 0xff, // mov    $0xfffffffffffffff2,%rax
			0xe9, 0xc0, 0xdd, 0xb7, 0x00, //             jmp    ffffffff81e01bc0 <__x86_return_thunk>
			0x48, 0x29, 0xd0, //                         sub    %rdx,%rax
			0x41, 0x55, //                               push   %r13
			0x41, 0x54, //                               push   %r12
			0x55,             //                         push   %rbp
			0x48, 0x89, 0xf5, //                         mov    %rsi,%rbp
			0x53,             //                         push   %rbx
			0x48, 0x89, 0xd3, //                         mov    %rdx,%rbx
			0x48, 0x39, 0xf0, //                         cmp    %rsi,%rax
			0x72, 0x52, //                               jb     ffffffff81283e66 <copy_from_user_nofault+0x86>
			0x49, 0x89, 0xfd, //                         mov    %rdi,%r13
			0xe8, 0xb4, 0x2b, 0xe0, 0xff, //             call   ffffffff810869d0 <nmi_uaccess_okay>
			0x84, 0xc0, //                               test   %al,%al
			0x74, 0x46, //                               je     ffffffff81283e66 <copy_from_user_nofault+0x86>
		},
	},
	"Amazon Linux - 6.1.56-82.125.amzn2023.x86_64": {
		isPatched:               true,
		copyFromUserNofaultAddr: 18446744071581352080, // 0xffffffff81264090
		nmiUaccessOkayAddr:      18446744071579331344, // 0xffffffff81076b10
		code: []byte{
			0xe8, 0x6b, 0xe1, 0xdf, 0xff, //             call   0xffffffff81062200
			0x48, 0xb8, 0x00, 0xf0, 0xff, 0xff, 0xff, // movabs $0x7ffffffff000,%rax
			0x7f, 0x00, 0x00,
			0x48, 0x39, 0xc2, //                         cmp    %rax,%rdx
			0x76, 0x0c, //                               jbe    0xffffffff812640b0
			0x48, 0xc7, 0xc0, 0xf2, 0xff, 0xff, 0xff, // mov    $0xfffffffffffffff2,%rax
			0xe9, 0x90, 0xea, 0xb9, 0x00, //             jmp    0xffffffff81e02b40
			0x48, 0x29, 0xd0, //                         sub    %rdx,%rax
			0x41, 0x55, //                               push   %r13
			0x41, 0x54, //                               push   %r12
			0x55,             //                         push   %rbp
			0x48, 0x89, 0xf5, //                         mov    %rsi,%rbp
			0x53,             //                         push   %rbx
			0x48, 0x89, 0xd3, //                         mov    %rdx,%rbx
			0x48, 0x39, 0xc6, //                         cmp    %rax,%rsi
			0x77, 0x52, //                               ja     0xffffffff81264116
			0x49, 0x89, 0xfd, //                         mov    %rdi,%r13
			0xe8, 0x44, 0x2a, 0xe1, 0xff, //             call   0xffffffff81076b10
			0x84, 0xc0, //                               test   %al,%al
			0x74, 0x46, //                               je     0xffffffff81264116
		},
	},
	"Debian - 5.19.0": {
		// https://snapshot.debian.org/archive/debian/20230501T024743Z/pool/main/l/linux/linux-image-5.19.0-0.deb11.2-cloud-amd64-dbg_5.19.11-1~bpo11%2B1_amd64.deb
		isPatched:               false,
		nmiUaccessOkayAddr:      18446744071579334128, // 0xffffffff810775f0
		copyFromUserNofaultAddr: 18446744071581280176, // 0xffffffff812527b0
		code: []byte{
			0xe8, 0x0b, 0x07, 0xe1, 0xff, //             call   ffffffff81062ec0 <__fentry__>
			0x48, 0xb8, 0x00, 0xf0, 0xff, 0xff, 0xff, // movabs $0x7ffffffff000,%rax
			0x7f, 0x00, 0x00,
			0x48, 0x39, 0xc2, //                         cmp    %rax,%rdx
			0x76, 0x0c, //                               jbe    ffffffff812527d0 <copy_from_user_nofault+0x20>
			0x48, 0xc7, 0xc0, 0xf2, 0xff, 0xff, 0xff, // mov    $0xfffffffffffffff2,%rax
			0xe9, 0x30, 0xf4, 0x9a, 0x00, //             jmp    ffffffff81c01c00 <__x86_return_thunk>
			0x48, 0x29, 0xd0, //                         sub    %rdx,%rax
			0x41, 0x55, //                               push   %r13
			0x49, 0x89, 0xf5, //                         mov    %rsi,%r13
			0x41, 0x54, //                               push   %r12
			0x55,             //                         push   %rbp
			0x53,             //                         push   %rbx
			0x48, 0x89, 0xd3, //                         mov    %rdx,%rbx
			0x48, 0x39, 0xc6, //                         cmp    %rax,%rsi
			0x76, 0x12, //                               jbe    ffffffff812527f6 <copy_from_user_nofault+0x46>
			0x5b,                                     // pop    %rbx
			0x48, 0xc7, 0xc0, 0xf2, 0xff, 0xff, 0xff, // mov    $0xfffffffffffffff2,%rax
			0x5d,       //                               pop    %rbp
			0x41, 0x5c, //                               pop    %r12
			0x41, 0x5d, //                               pop    %r13
		},
	},
}

func TestGetJumpInCopyFromUserNoFault(t *testing.T) {
	for name, test := range codeblobs {
		t.Run(name, func(t *testing.T) {
			isPatched, err := CopyFromUserNoFaultIsPatched(test.code,
				test.copyFromUserNofaultAddr, test.nmiUaccessOkayAddr)
			if assert.NoError(t, err) {
				assert.Equal(t, test.isPatched, isPatched)
			}
		})
	}
}
