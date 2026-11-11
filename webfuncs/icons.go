package webfuncs

import (
	"github.com/goldic/xtpl"

	"github.com/goldic/js"
)

// IconsCollection contains the icons used by the randIcon template function.
var IconsCollection = []string{
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "", "", "", "", "", "", "", "",
	"", "", "",
}

// ColorsCollection contains the colors used by the randColor template function.
var ColorsCollection = []string{
	"#ee0700", "#fa0003", "#ee0065", "#e100ec",
	"#6c00eb", "#2402eb", "#005aee", "#729de5",
	"#00b1ec", "#86ece0", "#00f48c", "#00d12d",
	"#f1f000", "#ffd752", "#ffc90b", "#f58444",
	"#666666", "#a7a7a7", "#d0d0d0",
}

var _ = xtpl.AddFunc("randIcon", func(seed any) string {
	return IconsCollection[randN(seed, len(IconsCollection))]
})

var _ = xtpl.AddFunc("randColor", func(seed any) string {
	return ColorsCollection[randN(seed, len(ColorsCollection))]
})

var _ = xtpl.AddFunc("hash64", func(seed any) uint64 {
	return Hash64(seed)
})

func randN(seed any, n int) int {
	return int(Hash64(seed)&0x7fffffffffffffff) % n
}

// Hash64 returns a deterministic hash of the encoded seed.
func Hash64(seed any) uint64 {
	data := js.Encode(seed)
	const (
		offset = 14695981039346656037
		prime1 = 1099511628211
		prime2 = 1073676287
	)
	var h uint64 = offset
	for _, c := range data {
		h = (h * prime1) ^ (uint64(c) * prime2)
	}
	return h
}
