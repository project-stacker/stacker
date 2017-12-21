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
	"testing"
)

func TestVisUnchanged(t *testing.T) {
	for _, test := range []struct {
		input string
		flag  VisFlag
	}{
		{"", DefaultVisFlags},
		{"helloworld", DefaultVisFlags},
		{"THIS_IS_A_TEST1234", DefaultVisFlags},
		{"SomeEncodingsAreCool", DefaultVisFlags},
		{"spaces are totally safe", DefaultVisFlags &^ VisSpace},
		{"tabs\tare\talso\tsafe!!", DefaultVisFlags &^ VisTab},
		{"just\a\atrustme\r\b\b!!", DefaultVisFlags | VisSafe},
	} {
		enc, err := Vis(test.input, test.flag)
		if err != nil {
			t.Errorf("unexpected error with %q: %s", test, err)
		}
		if enc != test.input {
			t.Errorf("expected encoding of %q (flag=%q) to be unchanged, got %q", test.input, test.flag, enc)
		}
	}
}

func TestVisFlags(t *testing.T) {
	for _, test := range []struct {
		input  string
		output string
		flag   VisFlag
	}{
		// Default
		{"AC_Ra\u00edz_Certic\u00e1mara_S.A..pem", "AC_Ra\\M-C\\M--z_Certic\\M-C\\M-!mara_S.A..pem", 0},
		{"z^i3i$\u00d3\u008anqgh5/t\u00e5<86>\u00b2kzla\\e^lv\u00df\u0093nv\u00df\u00aea|3}\u00d8\u0088\u00d6\u0084", `z^i3i$\M-C\M^S\M-B\M^Jnqgh5/t\M-C\M-%<86>\M-B\M-2kzla\\e^lv\M-C\M^_\M-B\M^Snv\M-C\M^_\M-B\M-.a|3}\M-C\M^X\M-B\M^H\M-C\M^V\M-B\M^D`, 0},
		{"@?e1xs+.R_Kjo]7s8pgRP:*nXCE4{!c", "@?e1xs+.R_Kjo]7s8pgRP:*nXCE4{!c", 0},
		{"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb", `62_\M-C\M^F\M-C\M^F2\M-B\M-.\M-B\M-7m\M-C\M^[\M-C\M^Cr^\M-B\M-?p\M-C\M^Fu'q\M-C\M-;c2\M-C\M-0u\M-B\M-8\M-C\M^]\M-C\M-(v\M-C\M-?\M-B\M-0\M-C\M^\\M-C\M^B\M-C\M-53\M-C\M^[-k\M-C\M-2sd4\\p\M-C\M^Z\M-B\M-&\M-C\M^S\M-C\M-.a<\M-C\M-&s{\M-B\240p\M-C\M-0\M-C\M-?j\M-C\240\M-C\M-(\M-B\M-8\M-B\M-8\M-B\M-<\M-C\M-<b`, 0},
		{"\u9003\"9v1)T798|o;fly jnKX\u0489Be=", `\M-i\M^@\M^C"9v1)T798|o;fly jnKX\M-R\M^IBe=`, 0},
		// VisOctal
		{"", "", VisOctal},
		{"\022", "\\022", VisOctal},
		{"\n \t", "\\012\\040\t", VisNewline | VisSpace | VisOctal},
		{"\x12\f\a\n\v\b  \U00012312", "\\022\\014\\007\n\\013\\010  \\360\\222\\214\\222", VisOctal},
		{"AC_Ra\u00edz_Certic\u00e1mara_S.A..pem", "AC_Ra\\303\\255z_Certic\\303\\241mara_S.A..pem", VisOctal},
		{"z^i3i$\u00d3\u008anqgh5/t\u00e5<86>\u00b2kzla\\e^lv\u00df\u0093nv\u00df\u00aea|3}\u00d8\u0088\u00d6\u0084", `z^i3i$\303\223\302\212nqgh5/t\303\245<86>\302\262kzla\\e^lv\303\237\302\223nv\303\237\302\256a|3}\303\230\302\210\303\226\302\204`, VisOctal},
		{"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb", `62_\303\206\303\2062\302\256\302\267m\303\233\303\203r^\302\277p\303\206u'q\303\273c2\303\260u\302\270\303\235\303\250v\303\277\302\260\303\234\303\202\303\2653\303\233-k\303\262sd4\\p\303\232\302\246\303\223\303\256a<\303\246s{\302\240p\303\260\303\277j\303\240\303\250\302\270\302\270\302\274\303\274b`, VisOctal},
		{"\u9003\"9v1)T798|o;fly jnKX\u0489Be=", `\351\200\203"9v1)T798|o;fly jnKX\322\211Be=`, VisOctal},
		// VisCStyle
		{"\x00 \f \a \n\v\b \r \t\r", "\\000 \\f \\a \n\\v\\b \\r \t\\r", VisCStyle},
		{"\t \n\v\b", "\\t \n\\v\\b", VisTab | VisCStyle},
		{"\n\v\t   ", "\n\\v\t\\s\\s\\s", VisSpace | VisCStyle},
		{"\n \n   ", "\\n \\n   ", VisNewline | VisCStyle},
		{"z^i3i$\u00d3\u008anqgh5/t\u00e5<86>\u00b2kzla\\e^lv\u00df\u0093nv\u00df\u00aea|3}\u00d8\u0088\u00d6\u0084", `z^i3i$\M-C\M^S\M-B\M^Jnqgh5/t\M-C\M-%<86>\M-B\M-2kzla\\e^lv\M-C\M^_\M-B\M^Snv\M-C\M^_\M-B\M-.a|3}\M-C\M^X\M-B\M^H\M-C\M^V\M-B\M^D`, VisCStyle},
		{"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb", `62_\M-C\M^F\M-C\M^F2\M-B\M-.\M-B\M-7m\M-C\M^[\M-C\M^Cr^\M-B\M-?p\M-C\M^Fu'q\M-C\M-;c2\M-C\M-0u\M-B\M-8\M-C\M^]\M-C\M-(v\M-C\M-?\M-B\M-0\M-C\M^\\M-C\M^B\M-C\M-53\M-C\M^[-k\M-C\M-2sd4\\p\M-C\M^Z\M-B\M-&\M-C\M^S\M-C\M-.a<\M-C\M-&s{\M-B\240p\M-C\M-0\M-C\M-?j\M-C\240\M-C\M-(\M-B\M-8\M-B\M-8\M-B\M-<\M-C\M-<b`, VisCStyle},
		{"\u9003\"9v1)T798|o;fly jnKX\u0489Be=", `\M-i\M^@\M^C"9v1)T798|o;fly\sjnKX\M-R\M^IBe=`, VisCStyle | VisSpace},
		// VisSpace
		{"  ", `\040\040`, VisSpace},
		{"\t \t", "\t\\040\t", VisSpace},
		{"\\040   plenty of characters here   ", `\\040\040\040\040plenty\040of\040characters\040here\040\040\040`, VisSpace},
		{"Js9L\u00cd\u00b2o?4824y'$|P}FIr%mW /KL9$]~", `Js9L\M-C\M^M\M-B\M-2o?4824y'$|P}FIr%mW\040/KL9$]~`, VisWhite},
		{"1\u00c6\u00abTcz+Vda?)k1%\\\"P;`po`h", `1\M-C\M^F\M-B\M-+Tcz+Vda?)k1%\\"P;` + "`po`" + `h`, VisWhite},
		{"\u9003\"9v1)T798|o;fly jnKX\u0489Be=", `\M-i\M^@\M^C"9v1)T798|o;fly\040jnKX\M-R\M^IBe=`, VisSpace},
		// VisTab
		{"\t  \v", "\\^I  \\^K", VisTab},
		{"\t  \v", "\\011  \\013", VisTab | VisOctal},
		// VisNewline
		{"\t\n  \v\r\n", "\t\\^J  \\^K\\^M\\^J", VisNewline},
		{"\t\n  \v\r\n", "\t\\012  \\013\\015\\012", VisNewline | VisOctal},
		// VisSafe
		// VisHTTPStyle
		{"\x12\f\a\n\v\b  \U00012312", `%12%0C%07%0A%0B%08%20%20%F0%92%8C%92`, VisHTTPStyle},
		{"1\u00c6\u00abTcz+Vda?)k1%\\\"P;`po`h", `1%C3%86%C2%ABTcz+Vda%3F)k1%25%5C%22P%3B%60po%60h`, VisHTTPStyle},
		{"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb", `62_%C3%86%C3%862%C2%AE%C2%B7m%C3%9B%C3%83r%5E%C2%BFp%C3%86u'q%C3%BBc2%C3%B0u%C2%B8%C3%9D%C3%A8v%C3%BF%C2%B0%C3%9C%C3%82%C3%B53%C3%9B-k%C3%B2sd4%5Cp%C3%9A%C2%A6%C3%93%C3%AEa%3C%C3%A6s%7B%C2%A0p%C3%B0%C3%BFj%C3%A0%C3%A8%C2%B8%C2%B8%C2%BC%C3%BCb`, VisHTTPStyle},
		{"'3Ze\u050e|\u02del\u069du-Rpct4+Z5b={@_{b", `'3Ze%D4%8E%7C%CB%9El%DA%9Du-Rpct4+Z5b%3D%7B%40_%7Bb`, VisHTTPStyle},
		// VisGlob
		{"cat /proc/**/status | grep '[pid]' ;; # cool code here", `cat /proc/\052\052/status | grep '\133pid]' ;; \043 cool code here`, VisGlob},
		{"@?e1xs+.R_Kjo]7s8pgRP:*nXCE4{!c", `@\077e1xs+.R_Kjo]7s8pgRP:\052nXCE4{!c`, VisGlob},
		{"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb", `62_\M-C\M^F\M-C\M^F2\M-B\M-.\M-B\M-7m\M-C\M^[\M-C\M^Cr^\M-B\M-?p\M-C\M^Fu'q\M-C\M-;c2\M-C\M-0u\M-B\M-8\M-C\M^]\M-C\M-(v\M-C\M-?\M-B\M-0\M-C\M^\\M-C\M^B\M-C\M-53\M-C\M^[-k\M-C\M-2sd4\\p\M-C\M^Z\M-B\M-&\M-C\M^S\M-C\M-.a<\M-C\M-&s{\M-B\240p\M-C\M-0\M-C\M-?j\M-C\240\M-C\M-(\M-B\M-8\M-B\M-8\M-B\M-<\M-C\M-<b`, VisGlob},
		{"62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u'q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db-k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb", `62_\303\206\303\2062\302\256\302\267m\303\233\303\203r^\302\277p\303\206u'q\303\273c2\303\260u\302\270\303\235\303\250v\303\277\302\260\303\234\303\202\303\2653\303\233-k\303\262sd4\\p\303\232\302\246\303\223\303\256a<\303\246s{\302\240p\303\260\303\277j\303\240\303\250\302\270\302\270\302\274\303\274b`, VisGlob | VisOctal},
		{"'3Ze\u050e|\u02del\u069du-Rpct4+Z5b={@_{b", `'3Ze\M-T\M^N|\M-K\M^^l\M-Z\M^]u-Rpct4+Z5b={@_{b`, VisGlob},
		{"'3Ze\u050e|\u02del\u069du-Rpct4+Z5b={@_{b", `'3Ze\324\216|\313\236l\332\235u-Rpct4+Z5b={@_{b`, VisGlob | VisOctal},
	} {
		enc, err := Vis(test.input, test.flag)
		if err != nil {
			t.Errorf("unexpected error with %q: %s", test, err)
		}
		if enc != test.output {
			t.Errorf("expected vis(%q, flag=%b) = %q, got %q", test.input, test.flag, test.output, enc)
		}
	}
}

func TestVisChanged(t *testing.T) {
	for _, test := range []string{
		"hello world",
		"THIS\\IS_A_TEST1234",
		"AC_Ra\u00edz_Certic\u00e1mara_S.A..pem",
	} {
		enc, err := Vis(test, DefaultVisFlags)
		if err != nil {
			t.Errorf("unexpected error with %q: %s", test, err)
		}
		if enc == test {
			t.Errorf("expected encoding of %q to be changed", test)
		}
	}
}
