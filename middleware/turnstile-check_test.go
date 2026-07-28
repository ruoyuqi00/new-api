/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldBypassTurnstileOnlyForGlobalAuthEntryPoints(t *testing.T) {
	tests := []struct {
		name string
		host string
		path string
		want bool
	}{
		{name: "global_login", host: "global.yuaiapi.com", path: "/api/user/login", want: true},
		{name: "global_login_with_port", host: "global.yuaiapi.com:443", path: "/api/user/login", want: true},
		{name: "global_register", host: "global.yuaiapi.com", path: "/api/user/register", want: true},
		{name: "global_email_verification", host: "global.yuaiapi.com", path: "/api/verification", want: true},
		{name: "main_login", host: "yuaiapi.com", path: "/api/user/login", want: false},
		{name: "main_register", host: "yuaiapi.com", path: "/api/user/register", want: false},
		{name: "global_password_reset", host: "global.yuaiapi.com", path: "/api/reset_password", want: false},
		{name: "global_profile", host: "global.yuaiapi.com", path: "/api/user/self", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, shouldBypassTurnstile(test.host, test.path))
		})
	}
}
