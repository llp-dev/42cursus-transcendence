// Command seedfull populates a running ft_transcendence database with a rich,
// self-contained demo dataset by writing directly to Postgres (via GORM),
// reusing the application's own models so the schema can never drift.
//
// Unlike scripts/seed.py (which drives the public REST API and therefore can't
// create direct messages — chat is WebSocket-only), this seeder writes every
// table the app uses, including private chat messages, post and comment
// reactions (like/dislike), hashtags, and 2FA, plus a fixed demo account with
// everything pre-populated around it.
//
// The two seeders are complementary and use disjoint email domains, so they
// don't collide: seed.py users live under their own domain, these under
// SEED_DOMAIN (default "demo.trans").
//
// Usage (from a context that can reach the DB, e.g. the seedfull compose
// service):
//
//	go run ./cmd/seedfull
//
// Idempotent: if the demo account already exists it exits without touching the
// data. Start from fresh volumes (make down/up) to re-seed.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"

	"ft_transcendence/backend/internal/config"
	"ft_transcendence/backend/internal/models"
	"ft_transcendence/backend/internal/utils"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const (
	userCount   = 50
	seedDomain  = "demo.trans"     // email domain for this seeder's users
	demoUser    = "demo"           // fixed, memorable demo login
	demoPass    = "Demo1234!"      // demo account password (bcrypt-hashed below)
	commonPass  = "SeedPass123!"   // password shared by the other 49 users
	twoFAUser   = "secure_sam"     // one account with 2FA enabled for the demo
	issuerName  = "ft_transcendence"
)

// activity tiers shape how much each user posts / how many likes they attract,
// so the leaderboard and gamification levels come out varied rather than flat.
type tier int

const (
	tierStar tier = iota // very active, lots of likes received
	tierActive
	tierQuiet
)

var rng = rand.New(rand.NewSource(42)) // fixed seed → reproducible dataset

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func pick[T any](s []T) T { return s[rng.Intn(len(s))] }

func hash(pw string) string {
	h, err := utils.HashString(pw)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	return h
}

func ptr[T any](v T) *T { return &v }

func dicebear(seed string) string {
	return "https://api.dicebear.com/9.x/avataaars/svg?seed=" + seed
}

func picsum(n int) string {
	return fmt.Sprintf("https://picsum.photos/seed/post%d/600/400", n)
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

type seededUser struct {
	model models.User
	tier  tier
}

func seedUsers(db *gorm.DB) []seededUser {
	log.Println("Seeding users…")
	out := make([]seededUser, 0, userCount)
	now := time.Now().UTC()

	for i := 0; i < userCount; i++ {
		var username, email, password string
		switch {
		case i == 0:
			username, email, password = demoUser, demoUser+"@"+seedDomain, demoPass
		default:
			username = fmt.Sprintf("%s%02d", strings.ToLower(strings.Split(displayNames[i], " ")[0]), i)
			email = fmt.Sprintf("%s@%s", username, seedDomain)
			password = commonPass
		}

		// Activity tier: first few users (incl. demo) are stars, then active,
		// a tail of quiet accounts.
		var t tier
		switch {
		case i < 4:
			t = tierStar
		case i < 40:
			t = tierActive
		default:
			t = tierQuiet
		}

		dob := now.AddDate(-(18 + rng.Intn(20)), 0, 0)
		u := models.User{
			ID:          utils.NewID(),
			Provider:    "local",
			DisplayName: displayNames[i],
			Username:    username,
			Email:       email,
			Password:    ptr(hash(password)),
			DateOfBirth: &dob,
			Bio:         bios[i%len(bios)],
			Avatar:      ptr(dicebear(username)),
			Wallpaper:   ptr(picsum(1000 + i)),
			CreatedAt:   now.Add(-time.Duration(userCount-i) * time.Hour),
			UpdatedAt:   now,
		}

		// One account gets real 2FA so the login-with-code path is demoable.
		if username == twoFAUser {
			key, err := totp.Generate(totp.GenerateOpts{Issuer: issuerName, AccountName: email})
			if err == nil {
				u.TwoFASecret = ptr(key.Secret())
				u.TwoFAEnabled = true
			}
		}

		if err := db.Create(&u).Error; err != nil {
			log.Fatalf("create user %s: %v", username, err)
		}
		out = append(out, seededUser{model: u, tier: t})
	}
	log.Printf("  → %d users created (demo=%s)\n", len(out), demoUser)
	return out
}

// ---------------------------------------------------------------------------
// Social graph: friendships (accepted + pending) and follows
// ---------------------------------------------------------------------------

func seedSocial(db *gorm.DB, users []seededUser) {
	log.Println("Seeding friendships & follows…")
	n := len(users)
	accepted, pending, follows := 0, 0, 0

	// Friendships: each user is accepted-friends with the next 2 (cyclic), and
	// has a pending request to the user 3 ahead. The demo user gets a few extra
	// accepted friends so its friend list looks alive.
	mkFriend := func(a, b, status string) {
		if err := db.Create(&models.Friend{
			ID: uuid.NewString(), UserID: a, FriendID: b, Status: status,
		}).Error; err != nil {
			log.Printf("  ! friend %s→%s: %v", a, b, err)
		}
	}
	for i := 0; i < n; i++ {
		a := users[i].model.ID
		mkFriend(a, users[(i+1)%n].model.ID, "accepted")
		accepted++
		mkFriend(a, users[(i+2)%n].model.ID, "accepted")
		accepted++
		mkFriend(a, users[(i+3)%n].model.ID, "pending")
		pending++
	}

	// Follows: everyone follows the stars; stars follow a handful back.
	for i := 0; i < n; i++ {
		follower := users[i].model.ID
		for s := 0; s < 4; s++ { // first 4 users are stars
			if i == s {
				continue
			}
			mkFriend(follower, users[s].model.ID, "follow")
			follows++
		}
		// plus a couple of random follows for variety
		for k := 0; k < 2; k++ {
			t := rng.Intn(n)
			if t != i {
				mkFriend(follower, users[t].model.ID, "follow")
				follows++
			}
		}
	}
	log.Printf("  → %d accepted, %d pending, %d follows\n", accepted, pending, follows)
}

// ---------------------------------------------------------------------------
// Posts (with hashtags + some media), then reactions & comments
// ---------------------------------------------------------------------------

func genPostContent() string {
	s := fmt.Sprintf("%s %s. %s", pick(postOpeners), pick(postNouns), pick(postTails))
	switch rng.Intn(3) {
	case 1:
		s += " " + pick(tags)
	case 2:
		s += " " + pick(tags) + " " + pick(tags)
	}
	return s
}

func postsForTier(t tier) int {
	switch t {
	case tierStar:
		return 8 + rng.Intn(8) // 8-15
	case tierActive:
		return 2 + rng.Intn(5) // 2-6
	default:
		return rng.Intn(2) // 0-1
	}
}

func seedPosts(db *gorm.DB, users []seededUser) []models.Post {
	log.Println("Seeding posts…")
	var posts []models.Post
	mediaCounter := 0
	now := time.Now().UTC()

	for _, su := range users {
		count := postsForTier(su.tier)
		for j := 0; j < count; j++ {
			content := genPostContent()
			p := models.Post{
				ID:        utils.NewID(),
				AuthorID:  su.model.ID,
				Content:   content,
				Tags:      pq.StringArray(utils.ExtractHashtags(content)),
				CreatedAt: now.Add(-time.Duration(rng.Intn(60*24)) * time.Hour),
				UpdatedAt: now,
			}
			// ~15% of posts carry an image.
			if rng.Intn(100) < 15 {
				p.MediaURL = ptr(picsum(mediaCounter))
				p.MediaMIME = ptr("image/jpeg")
				mediaCounter++
			}
			if err := db.Create(&p).Error; err != nil {
				log.Printf("  ! post: %v", err)
				continue
			}
			posts = append(posts, p)
		}
	}
	log.Printf("  → %d posts (%d with media)\n", len(posts), mediaCounter)
	return posts
}

// seedReactions creates like/dislike rows in the "likes" table and updates the
// denormalized counters on each post. Stars' posts attract far more reactions.
func seedReactions(db *gorm.DB, users []seededUser, posts []models.Post) {
	log.Println("Seeding post reactions (like/dislike)…")
	likes, dislikes := 0, 0

	// Map authorID → tier for weighting.
	tierOf := map[string]tier{}
	for _, u := range users {
		tierOf[u.model.ID] = u.tier
	}

	for _, p := range posts {
		// Reaction probability higher for star-authored posts.
		base := 15
		if tierOf[p.AuthorID] == tierStar {
			base = 55
		}
		var l, d int
		for _, u := range users {
			if u.model.ID == p.AuthorID {
				continue
			}
			if rng.Intn(100) >= base {
				continue
			}
			value := 1
			if rng.Intn(100) < 20 { // ~20% of reactions are dislikes
				value = -1
			}
			r := models.PostReaction{
				ID: utils.NewID(), UserID: u.model.ID, PostID: p.ID,
				Value: value, CreatedAt: time.Now().UTC(),
			}
			if err := db.Create(&r).Error; err != nil {
				continue
			}
			if value == 1 {
				l++
				likes++
			} else {
				d++
				dislikes++
			}
		}
		db.Model(&models.Post{}).Where("id = ?", p.ID).
			Updates(map[string]any{"likes_count": l, "dislikes_count": d})
	}
	log.Printf("  → %d likes, %d dislikes\n", likes, dislikes)
}

func seedComments(db *gorm.DB, users []seededUser, posts []models.Post) []models.Reply {
	log.Println("Seeding comments…")
	var replies []models.Reply
	for _, p := range posts {
		count := rng.Intn(5) // 0-4 comments
		for j := 0; j < count; j++ {
			author := users[rng.Intn(len(users))].model
			r := models.Reply{
				ID: utils.NewID(), PostID: p.ID, AuthorID: author.ID,
				Content: pick(comments), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}
			if err := db.Create(&r).Error; err != nil {
				continue
			}
			replies = append(replies, r)
		}
		db.Model(&models.Post{}).Where("id = ?", p.ID).
			Update("comments_count", count)
	}
	log.Printf("  → %d comments\n", len(replies))
	return replies
}

func seedCommentReactions(db *gorm.DB, users []seededUser, replies []models.Reply) {
	log.Println("Seeding comment reactions…")
	likes, dislikes := 0, 0
	for _, r := range replies {
		var l, d int
		for _, u := range users {
			if u.model.ID == r.AuthorID || rng.Intn(100) >= 20 {
				continue
			}
			value := 1
			if rng.Intn(100) < 25 {
				value = -1
			}
			rr := models.ReplyReaction{
				ID: utils.NewID(), UserID: u.model.ID, ReplyID: r.ID,
				Value: value, CreatedAt: time.Now().UTC(),
			}
			if err := db.Create(&rr).Error; err != nil {
				continue
			}
			if value == 1 {
				l++
				likes++
			} else {
				d++
				dislikes++
			}
		}
		db.Model(&models.Reply{}).Where("id = ?", r.ID).
			Updates(map[string]any{"likes_count": l, "dislikes_count": d})
	}
	log.Printf("  → %d comment likes, %d dislikes\n", likes, dislikes)
}

// ---------------------------------------------------------------------------
// Private chat (DM). Written straight to the messages table — the only way to
// pre-populate chat, since sending goes through WebSocket at runtime.
// ---------------------------------------------------------------------------

func seedMessages(db *gorm.DB, users []seededUser) {
	log.Println("Seeding direct messages…")
	convos, msgs := 0, 0

	mkConversation := func(a, b seededUser, lines int) {
		base := time.Now().UTC().Add(-time.Duration(rng.Intn(48)) * time.Hour)
		for k := 0; k < lines; k++ {
			sender, recipient := a, b
			if k%2 == 1 {
				sender, recipient = b, a
			}
			id, err := uuid.NewV7()
			if err != nil {
				continue
			}
			m := models.Message{
				ID:          id.String(),
				SenderID:    sender.model.ID,
				Username:    sender.model.Username,
				RecipientID: recipient.model.ID,
				Content:     pick(dmLines),
				Type:        "dm",
				CreatedAt:   base.Add(time.Duration(k) * time.Minute),
			}
			if err := db.Create(&m).Error; err != nil {
				continue
			}
			msgs++
		}
		convos++
	}

	// The demo user (index 0) gets several active conversations with its
	// friends so the chat panel is full on first login.
	demo := users[0]
	for i := 1; i <= 5 && i < len(users); i++ {
		mkConversation(demo, users[i], 5+rng.Intn(6))
	}
	// Plus ~15 conversations between random other pairs.
	for c := 0; c < 15; c++ {
		a := users[rng.Intn(len(users))]
		b := users[rng.Intn(len(users))]
		if a.model.ID == b.model.ID {
			continue
		}
		mkConversation(a, b, 3+rng.Intn(5))
	}
	log.Printf("  → %d conversations, %d messages\n", convos, msgs)
}

// ---------------------------------------------------------------------------
// Notifications (varied types, some unread)
// ---------------------------------------------------------------------------

func seedNotifications(db *gorm.DB, users []seededUser, posts []models.Post) {
	log.Println("Seeding notifications…")
	n := len(users)
	count := 0

	mk := func(recipient, actor seededUser, typ, content, postID string, read bool) {
		notif := models.Notification{
			ID:            uuid.NewString(),
			CreatedAt:     time.Now().UTC().Add(-time.Duration(rng.Intn(72)) * time.Hour),
			UserID:        recipient.model.ID,
			UserUsername:  recipient.model.Username,
			ActorID:       actor.model.ID,
			ActorUsername: actor.model.Username,
			Type:          typ,
			Content:       content,
			PostID:        postID,
			Read:          read,
		}
		if err := db.Create(&notif).Error; err == nil {
			count++
		}
	}

	for i := 0; i < n; i++ {
		recipient := users[i]
		actor := users[(i+1)%n]
		// a friend request and a follow, half of them unread
		mk(recipient, actor, "friend_request", actor.model.Username+" sent you a friend request", "", i%2 == 0)
		mk(recipient, users[(i+2)%n], "follow", users[(i+2)%n].model.Username+" started following you", "", i%3 == 0)
	}

	// Like / reply notifications tied to real posts, aimed mostly at the demo
	// user so its bell is full and partly unread.
	demo := users[0]
	for _, p := range posts {
		if p.AuthorID != demo.model.ID {
			continue
		}
		actor := users[1+rng.Intn(n-1)]
		mk(demo, actor, "like", actor.model.Username+" liked your post", p.ID, false)
		if rng.Intn(2) == 0 {
			mk(demo, actor, "reply", actor.model.Username+" commented on your post", p.ID, false)
		}
	}
	log.Printf("  → %d notifications\n", count)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := cfg.Postgres.Connect() // runs AutoMigrate, guaranteeing the schema
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	// Idempotence: bail out if the demo account already exists.
	var existing int64
	db.Model(&models.User{}).Where("email = ?", demoUser+"@"+seedDomain).Count(&existing)
	if existing > 0 {
		log.Printf("Demo account already present (%s@%s) — nothing to do.", demoUser, seedDomain)
		log.Printf("To re-seed, recreate the volumes (make down && make up) then run again.")
		os.Exit(0)
	}

	log.Println("=== seedfull: building demo dataset ===")
	users := seedUsers(db)
	seedSocial(db, users)
	posts := seedPosts(db, users)
	seedReactions(db, users, posts)
	replies := seedComments(db, users, posts)
	seedCommentReactions(db, users, replies)
	seedMessages(db, users)
	seedNotifications(db, users, posts)

	log.Println("=== seedfull complete ===")
	log.Printf("Demo login:  %s@%s  /  %s", demoUser, seedDomain, demoPass)
	log.Printf("Other users: <name><nn>@%s  /  %s", seedDomain, commonPass)
	log.Printf("2FA account: %s@%s (TOTP enabled)", twoFAUser, seedDomain)
}
