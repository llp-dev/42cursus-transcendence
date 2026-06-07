package main

// data.go — content pools used by the seeder. Kept separate from the seeding
// logic so the prose (names, bios, post fragments) is easy to tweak without
// touching the insertion code.

// Display names for the 50 seeded users. The first entry is overridden by the
// fixed demo account; the rest are assigned in order.
var displayNames = []string{
	"Demo User", "Alice Dupont", "Carlos Méndez", "Sophie Martin", "John Smith",
	"Emma Garcia", "Lucas Silva", "Mia Chen", "Ahmed Ali", "Olivia Brown",
	"Noah Wilson", "Léa Moreau", "Hugo Bernard", "Chloé Petit", "Liam O'Brien",
	"Yuki Tanaka", "Sara Costa", "Marco Rossi", "Nina Schmidt", "Tom Andersen",
	"Priya Patel", "Diego Torres", "Anna Kowalski", "Felix Weber", "Maya Nguyen",
	"Omar Haddad", "Ella Johansson", "Ravi Kumar", "Clara Fischer", "Leo Martins",
	"Zoe Williams", "Adam Novak", "Layla Hassan", "Ben Carter", "Isla Murphy",
	"Kenji Sato", "Nora Larsen", "Pablo Ortiz", "Greta Hoffmann", "Sam Taylor",
	"Aisha Khan", "Victor Lambert", "Ivy Robinson", "Theo Dubois", "Lina Berg",
	"Max Schäfer", "Ruby Evans", "Yara Saab", "Finn Walsh", "Tara Singh",
}

// Bios assigned round-robin to users.
var bios = []string{
	"Loves photography and travel ✈️",
	"Backend developer · Go & Postgres",
	"UI/UX designer with a coffee problem",
	"Tech enthusiast and weekend hiker",
	"Frontend dev · React all day",
	"Open-source contributor 🐧",
	"Building things that scale",
	"Debugging in production since forever",
	"Cat person, vim user",
	"Always learning something new",
	"Marathon runner & data nerd",
	"Music, code, repeat 🎧",
	"Cloud, containers, chaos",
	"Designing calm interfaces",
	"Ask me about distributed systems",
}

// Post fragments composed into varied content. Each generated post mixes these
// with 0-2 hashtags so the trends aggregation has something to rank.
var postOpeners = []string{
	"Just shipped", "Finally finished", "Spent all day on", "Refactored",
	"Deep-diving into", "Experimenting with", "Learning about", "Rewrote",
}
var postNouns = []string{
	"the auth flow", "our CI pipeline", "the websocket layer", "a new feature",
	"the database schema", "the caching strategy", "our test suite", "the deploy script",
}
var postTails = []string{
	"Pretty happy with how it turned out.",
	"Still a few edge cases to handle.",
	"Coffee count: too high.",
	"Feedback welcome!",
	"Took longer than expected, of course.",
	"Small win, but a win.",
	"On to the next thing.",
	"Who else is working on this?",
}

// Hashtags sprinkled into posts. Lowercase to match the backend's
// case-insensitive extraction.
var tags = []string{
	"#golang", "#react", "#devops", "#postgres", "#docker", "#42born2code",
	"#webdev", "#opensource", "#coding", "#typescript", "#kubernetes", "#redis",
}

// Comment bodies picked at random.
var comments = []string{
	"Great work!", "This is really clean.", "How did you handle the edge cases?",
	"Saving this for later.", "Same experience here.", "Love the approach 👏",
	"Could you share the repo?", "Interesting, hadn't thought of that.",
	"Nice one!", "This helped me a lot, thanks.", "Solid explanation.",
	"Bookmarked.",
}

// Direct-message lines used to build realistic conversations.
var dmLines = []string{
	"hey, you around?",
	"did you see the latest PR?",
	"can you review my branch when you get a sec?",
	"lunch later?",
	"the build is green again 🎉",
	"I pushed the fix, should be good now",
	"thanks for the help earlier!",
	"meeting moved to 3pm",
	"check the logs, there's a weird error",
	"nice, that worked perfectly",
	"let's pair on this tomorrow",
	"merged, you can pull main",
}
