/*
 * govis: unicode aware vis(3) encoding implementation
 * Copyright (C) 2017 SUSE LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package govis

import (
	"bytes"
	"crypto/rand"
	"testing"
)

const DefaultVisFlags = VisWhite | VisOctal | VisGlob

func TestRandomVisUnvis(t *testing.T) {
	// Randomly generate N strings.
	const N = 100

	for i := 0; i < N; i++ {
		testBytes := make([]byte, 256)
		if n, err := rand.Read(testBytes); n != cap(testBytes) || err != nil {
			t.Fatalf("could not read enough bytes: err=%v n=%d", err, n)
		}
		test := string(testBytes)

		for flag := VisFlag(0); flag <= visMask; flag++ {
			// VisNoSlash is frankly just a dumb flag, and it is impossible for us
			// to actually preserve things in a round-trip.
			if flag&VisNoSlash == VisNoSlash {
				continue
			}

			enc, err := Vis(test, flag)
			if err != nil {
				t.Errorf("unexpected error doing vis(%q, %b): %s", test, flag, err)
				continue
			}
			dec, err := Unvis(enc, flag)
			if err != nil {
				t.Errorf("unexpected error doing unvis(%q, %b): %s", enc, flag, err)
				continue
			}
			if dec != test {
				t.Errorf("roundtrip failed: unvis(vis(%q, %b) = %q, %b) = %q", test, flag, enc, flag, dec)
			}
		}
	}
}

func TestRandomVisVisUnvisUnvis(t *testing.T) {
	// Randomly generate N strings.
	const N = 100

	for i := 0; i < N; i++ {
		testBytes := make([]byte, 256)
		if n, err := rand.Read(testBytes); n != cap(testBytes) || err != nil {
			t.Fatalf("could not read enough bytes: err=%v n=%d", err, n)
		}
		test := string(testBytes)

		for flag := VisFlag(0); flag <= visMask; flag++ {
			// VisNoSlash is frankly just a dumb flag, and it is impossible for us
			// to actually preserve things in a round-trip.
			if flag&VisNoSlash == VisNoSlash {
				continue
			}

			enc, err := Vis(test, flag)
			if err != nil {
				t.Errorf("unexpected error doing vis(%q, %b): %s", test, flag, err)
				continue
			}
			enc2, err := Vis(enc, flag)
			if err != nil {
				t.Errorf("unexpected error doing vis(%q, %b): %s", enc, flag, err)
				continue
			}
			dec, err := Unvis(enc2, flag)
			if err != nil {
				t.Errorf("unexpected error doing unvis(%q, %b): %s", enc2, flag, err)
				continue
			}
			dec2, err := Unvis(dec, flag)
			if err != nil {
				t.Errorf("unexpected error doing unvis(%q, %b): %s", dec, flag, err)
				continue
			}
			if dec2 != test {
				t.Errorf("roundtrip failed: unvis(unvis(vis(vis(%q) = %q) = %q) = %q, %b) = %q", test, enc, enc2, dec, flag, dec2)
			}
		}
	}
}

func TestVisUnvis(t *testing.T) {
	for flag := VisFlag(0); flag <= visMask; flag++ {
		// VisNoSlash is frankly just a dumb flag, and it is impossible for us
		// to actually preserve things in a round-trip.
		if flag&VisNoSlash == VisNoSlash {
			continue
		}

		// Round-trip testing.
		for _, test := range []string{
			"",
			"hello world",
			"THIS\\IS_A_TEST1234",
			"this.is.a.normal_string",
			"AC_Ra\u00edz_Certic\u00e1mara_S.A..pem",
			"NetLock_Arany_=Class_Gold=_F\u0151tan\u00fas\u00edtv\u00e1ny.pem",
			"T\u00dcB\u0130TAK_UEKAE_K\u00f6k_Sertifika_Hizmet_Sa\u011flay\u0131c\u0131s\u0131_-_S\u00fcr\u00fcm_3.pem",
			"hello world [ this string needs=enco ding! ]",
			"even \n more encoding necessary\a\a ",
			"\024 <-- some more weird characters --> \u4f60\u597d\uff0c\u4e16\u754c",
			"\\xff\\n double encoding is also great fun \\x",
			"AC_Ra\\M-C\\M--z_Certic\\M-C\\M-!mara_S.A..pem",
			"z^i3i$\u00d3\u008anqgh5/t\u00e5<86>\u00b2kzla\\e^lv\u00df\u0093nv\u00df\u00aea|3}\u00d8\u0088\u00d6\u0084",
			`z^i3i$\M-C\M^S\M-B\M^Jnqgh5/t\M-C\M-%<86>\M-B\M-2kzla\\e^lv\M-C\M^_\M-B\M^Snv\M-C\M^_\M-B\M-.a|3}\M-C\M^X\M-B\M^H\M-C\M^V\M-B\M^D`,
			"@?e1xs+.R_Kjo]7s8pgRP:*nXCE4{!c",
			"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb",
			`62_\M-C\M^F\M-C\M^F2\M-B\M-.\M-B\M-7m\M-C\M^[\M-C\M^Cr^\M-B\M-?p\M-C\M^Fu'q\M-C\M-;c2\M-C\M-0u\M-B\M-8\M-C\M^]\M-C\M-(v\M-C\M-?\M-B\M-0\M-C\M^\\M-C\M^B\M-C\M-53\M-C\M^[-k\M-C\M-2sd4\\p\M-C\M^Z\M-B\M-&\M-C\M^S\M-C\M-.a<\M-C\M-&s{\M-B\240p\M-C\M-0\M-C\M-?j\M-C\240\M-C\M-(\M-B\M-8\M-B\M-8\M-B\M-<\M-C\M-<b`,
			"\u9003\"9v1)T798|o;fly jnKX\u0489Be=",
			`\M-i\M^@\M^C"9v1)T798|o;fly jnKX\M-R\M^IBe=`,
			"'3Ze\u050e|\u02del\u069du-Rpct4+Z5b={@_{b",
			`'3Ze\M-T\M^N|\M-K\M^^l\M-Z\M^]u-Rpct4+Z5b={@_{b`,
			"1\u00c6\u00abTcz+Vda?)k1%\\\"P;`po`h",
			`1%C3%86%C2%ABTcz+Vda%3F)k1%25%5C%22P%3B%60po%60h`,
		} {
			enc, err := Vis(test, flag)
			if err != nil {
				t.Errorf("unexpected error doing vis(%q, %b): %s", test, flag, err)
				continue
			}
			dec, err := Unvis(enc, flag)
			if err != nil {
				t.Errorf("unexpected error doing unvis(%q, %b): %s", enc, flag, err)
				continue
			}
			if dec != test {
				t.Errorf("roundtrip failed: unvis(vis(%q, %b) = %q, %b) = %q", test, flag, enc, flag, dec)
			}
		}
	}
}

func TestByteStrings(t *testing.T) {
	// It's important to make sure that we don't mess around with the layout of
	// bytes when doing a round-trip. Otherwise we risk outputting visually
	// identical but bit-stream non-identical strings (causing much confusion
	// when trying to access such files).

	for _, test := range [][]byte{
		[]byte("This is a man in business suit levitating: \U0001f574"),
		{0x7f, 0x17, 0x01, 0x33},
		// TODO: Test arbitrary byte streams like the one below. Currently this
		//       fails because Vis() is messing around with it (converting it
		//       to a rune and spacing it out).
		//{'\xef', '\xae', 'h', '\077', 'k'},
	} {
		testString := string(test)
		enc, err := Vis(testString, DefaultVisFlags)
		if err != nil {
			t.Errorf("unexpected error doing vis(%q): %s", test, err)
			continue
		}
		dec, err := Unvis(enc, DefaultVisFlags)
		if err != nil {
			t.Errorf("unexpected error doing unvis(%q): %s", enc, err)
			continue
		}
		decBytes := []byte(dec)

		if dec != testString {
			t.Errorf("roundtrip failed [string comparison]: unvis(vis(%q) = %q) = %q", test, enc, dec)
		}
		if !bytes.Equal(decBytes, test) {
			t.Errorf("roundtrip failed [byte comparison]: unvis(vis(%q) = %q) = %q", test, enc, dec)
		}
	}

}
