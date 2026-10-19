package ui

// IconSet maps meanings to glyphs. All three sets carry the same meanings, so
// switching sets never changes what a screen says (plan §10).
type IconSet struct {
	Name                                     string
	OK, Fail, Warn, Info, Step, Tree, Bullet string
	Process, Port, Disk, Memory, Agent, Star string
	Trash, Rocket, Party, Lock, Font, Theme  string
	Git, Docker, Clock, Arrow, Shield, Spark string
}

var emojiIcons = IconSet{
	Name: "emoji",
	OK:   "✔", Fail: "✖", Warn: "▲", Info: "ℹ", Step: "⏺", Tree: "⎿", Bullet: "•",
	Process: "●", Port: "⚡", Disk: "🧹", Memory: "🧠", Agent: "🤖", Star: "✦",
	Trash: "🗑️", Rocket: "🚀", Party: "🎉", Lock: "🔒", Font: "🔤", Theme: "🎨",
	Git: "🌿", Docker: "🐳", Clock: "⏱", Arrow: "→", Shield: "🛡️", Spark: "✨",
}

// Nerd Font v3 code points (https://www.nerdfonts.com/cheat-sheet).
var nerdIcons = IconSet{
	Name: "nerd",
	OK:   "\uf00c", Fail: "\uf00d", Warn: "\uf071", Info: "\uf05a", Step: "⏺", Tree: "⎿", Bullet: "\uf444",
	Process: "\uf013", Port: "\U000f0200", Disk: "\U000f00e2", Memory: "\U000f035b", Agent: "\U000f06a9", Star: "\uf005",
	Trash: "\uf1f8", Rocket: "\uf135", Party: "\uf0d0", Lock: "\uf023", Font: "\uf031", Theme: "\ue22b",
	Git: "\ue725", Docker: "\uf308", Clock: "\uf017", Arrow: "\uf061", Shield: "\U000f0498", Spark: "\uf0d0",
}

var asciiIcons = IconSet{
	Name: "ascii",
	OK:   "[ok]", Fail: "[x]", Warn: "[!]", Info: "[i]", Step: "*", Tree: "|_", Bullet: "-",
	Process: "*", Port: ":", Disk: "#", Memory: "@", Agent: ">", Star: "*",
	Trash: "[trash]", Rocket: ">>", Party: "!!", Lock: "[lock]", Font: "[font]", Theme: "[theme]",
	Git: "[git]", Docker: "[docker]", Clock: "~", Arrow: "->", Shield: "[safe]", Spark: "*",
}
