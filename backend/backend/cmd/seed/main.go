// Seed script for the Transcendence backend.
//
// Creates users from users.json plus example data:
//   - friend requests (some pending, some accepted)
//   - follows
//   - posts
//   - likes between users on posts
//   - replies (comments) on posts
//   - example notifications
//
// Idempotent: re-running the script doesn't duplicate existing rows.
//
// Usage: from backend/cmd/seed: `go run main.go`

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/Transcendence/config"
	"github.com/Transcendence/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// --- Data ---

var postContents = []string{
	"Just finished an amazing project! Feeling proud",
	"Anyone else love coding at 3am? Coffee is life",
	"Check out this cool new tech stack I'm learning about!",
	"Weekend is here! Time to build something cool",
	"Finally deployed to production! No more bugs (hopefully)",
	"Open source contributions are the best way to learn",
	"Just discovered this incredible library, game changer!",
	"Working on something secret, can't wait to share soon...",
	"The debugging journey never ends... but that's what makes it fun!",
	"New blog post is live! Check it out, feedback welcome",
	"Sometimes the best code is the code you delete",
	"Excited to announce we're hiring! Great team, great mission",
}

var commentContents = []string{
	"Great post!",
	"Totally agree.",
	"Could you share more details?",
	"Thanks for sharing!",
	"This is awesome!",
	"Same here haha",
	"Love this perspective.",
	"Interesting take!",
}

// --- Helpers ---

func hashPassword(password string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

type seedUserInput struct {
	Name        string    `json:"name"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	DateOfBirth time.Time `json:"dateOfBirth"`
	Bio         string    `json:"bio"`
	Wallpaper   string    `json:"wallpaper"`
	Avatar      string    `json:"avatar"`
}

func (s seedUserInput) toUser() models.User {
	hashed := hashPassword(s.Password)
	dob := s.DateOfBirth
	wallpaper := s.Wallpaper
	avatar := s.Avatar
	now := time.Now()

	return models.User{
		ID:          uuid.NewString(),
		Name:        s.Name,
		Username:    s.Username,
		Email:       s.Email,
		Password:    &hashed,
		DateOfBirth: &dob,
		Bio:         s.Bio,
		Wallpaper:   &wallpaper,
		Avatar:      &avatar,
		Provider:    "local",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// --- Seeders ---

func ensureSchema(db *gorm.DB) error {
	fmt.Println("Ensuring schema is up to date...")
	return db.AutoMigrate(
		&models.User{},
		&models.Friend{},
		&models.Post{},
		&models.Like{},
		&models.Reply{},
		&models.Repost{},
		&models.Message{},
		&models.Notification{},
		&models.File{},
		&models.FileAccess{},
	)
}

func seedUsers(db *gorm.DB) []models.User {
	fmt.Println("\nSeeding users...")
	file, err := os.ReadFile("users.json")
	if err != nil {
		panic(err)
	}

	var inputs []seedUserInput
	if err := json.Unmarshal(file, &inputs); err != nil {
		panic(err)
	}

	for _, in := range inputs {
		var existing models.User
		if err := db.Where("email = ? OR username = ?", in.Email, in.Username).First(&existing).Error; err == nil {
			fmt.Println("  - User already exists:", in.Email)
			continue
		}

		user := in.toUser()
		if err := db.Create(&user).Error; err != nil {
			fmt.Println("  - Error inserting user:", err)
		} else {
			fmt.Println("  + Inserted user:", user.Username)
		}
	}

	// Reload all users so the rest of the seeder works with fresh IDs
	var users []models.User
	db.Find(&users)
	return users
}

// seedFriendships creates a mix of accepted and pending friend requests.
// Pattern: user[i] is accepted friend with user[i+1] (cyclic),
// and user[i] has a pending request to user[i+2] (cyclic).
func seedFriendships(db *gorm.DB, users []models.User) {
	fmt.Println("\nSeeding friendships...")

	n := len(users)
	if n < 3 {
		fmt.Println("  - Need at least 3 users to seed friendships, skipping")
		return
	}

	accepted := 0
	pending := 0
	for i := 0; i < n; i++ {
		// Accepted: i ↔ (i+1) mod n
		acceptedTarget := users[(i+1)%n]
		if !friendExists(db, users[i].ID, acceptedTarget.ID) {
			db.Create(&models.Friend{
				ID:       uuid.NewString(),
				UserID:   users[i].ID,
				FriendID: acceptedTarget.ID,
				Status:   "accepted",
			})
			accepted++
		}

		// Pending: i → (i+2) mod n
		pendingTarget := users[(i+2)%n]
		if !friendExists(db, users[i].ID, pendingTarget.ID) {
			db.Create(&models.Friend{
				ID:       uuid.NewString(),
				UserID:   users[i].ID,
				FriendID: pendingTarget.ID,
				Status:   "pending",
			})
			pending++
		}
	}

	fmt.Printf("  + Created %d accepted friendships, %d pending requests\n", accepted, pending)
}

// seedFollows creates follow relationships.
// Each user follows the next 2 users (cyclic).
func seedFollows(db *gorm.DB, users []models.User) {
	fmt.Println("\nSeeding follows...")

	n := len(users)
	if n < 2 {
		fmt.Println("  - Need at least 2 users to seed follows, skipping")
		return
	}

	created := 0
	for i := 0; i < n; i++ {
		for offset := 1; offset <= 2 && offset < n; offset++ {
			target := users[(i+offset)%n]
			if friendExistsWithStatus(db, users[i].ID, target.ID, "follow") {
				continue
			}
			db.Create(&models.Friend{
				ID:       uuid.NewString(),
				UserID:   users[i].ID,
				FriendID: target.ID,
				Status:   "follow",
			})
			created++
		}
	}

	fmt.Printf("  + Created %d follow relationships\n", created)
}

// seedPosts creates a few posts per user.
func seedPosts(db *gorm.DB, users []models.User) []models.Post {
	fmt.Println("\nSeeding posts...")

	if len(users) == 0 {
		fmt.Println("  - No users found, skipping")
		return nil
	}

	for contentIdx, content := range postContents {
		user := users[contentIdx%len(users)]
		post := models.Post{
			ID:        uuid.NewString(),
			Content:   content,
			AuthorID:  user.ID,
			CreatedAt: time.Now().Add(-time.Duration((len(postContents)-contentIdx)*24) * time.Hour),
			UpdatedAt: time.Now().Add(-time.Duration((len(postContents)-contentIdx)*24) * time.Hour),
		}

		var existing models.Post
		if err := db.Where("content = ? AND author_id = ?", post.Content, post.AuthorID).First(&existing).Error; err == nil {
			continue
		}

		if err := db.Create(&post).Error; err != nil {
			fmt.Println("  - Error inserting post:", err)
		}
	}

	var posts []models.Post
	db.Find(&posts)
	fmt.Printf("  + Total posts: %d\n", len(posts))
	return posts
}

// seedLikes creates some likes by various users on various posts.
func seedLikes(db *gorm.DB, users []models.User, posts []models.Post) {
	fmt.Println("\nSeeding likes...")

	if len(users) == 0 || len(posts) == 0 {
		fmt.Println("  - Need users and posts, skipping")
		return
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	created := 0
	for _, post := range posts {
		// Each post gets liked by ~half the users (random subset)
		for _, user := range users {
			if user.ID == post.AuthorID {
				continue // don't auto-like
			}
			if rng.Float64() > 0.5 {
				continue
			}

			var existing models.Like
			if err := db.Where("post_id = ? AND user_id = ?", post.ID, user.ID).First(&existing).Error; err == nil {
				continue
			}

			db.Create(&models.Like{
				ID:     uuid.NewString(),
				PostID: post.ID,
				UserID: user.ID,
			})
			created++

			// Increment LikesCount on the post
			db.Model(&models.Post{}).
				Where("id = ?", post.ID).
				UpdateColumn("likes_count", gorm.Expr("likes_count + ?", 1))
		}
	}

	fmt.Printf("  + Created %d likes\n", created)
}

// seedReplies creates a few comments on each post.
func seedReplies(db *gorm.DB, users []models.User, posts []models.Post) {
	fmt.Println("\nSeeding replies (comments)...")

	if len(users) < 2 || len(posts) == 0 {
		fmt.Println("  - Need users and posts, skipping")
		return
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + 1))
	created := 0
	for i, post := range posts {
		// 1-3 comments per post
		count := 1 + rng.Intn(3)
		for j := 0; j < count; j++ {
			// Use a different user than the post author
			authorIdx := (i + j + 1) % len(users)
			author := users[authorIdx]
			if author.ID == post.AuthorID {
				authorIdx = (authorIdx + 1) % len(users)
				author = users[authorIdx]
			}

			content := commentContents[(i+j)%len(commentContents)]

			var existing models.Reply
			if err := db.Where("post_id = ? AND author_id = ? AND content = ?",
				post.ID, author.ID, content).First(&existing).Error; err == nil {
				continue
			}

			db.Create(&models.Reply{
				ID:       uuid.NewString(),
				PostID:   post.ID,
				AuthorID: author.ID,
				Content:  content,
			})
			created++

			db.Model(&models.Post{}).
				Where("id = ?", post.ID).
				UpdateColumn("comments_count", gorm.Expr("comments_count + ?", 1))
		}
	}

	fmt.Printf("  + Created %d replies\n", created)
}

// seedNotifications creates a couple of unread notifications per user so the UI
// can show the "you have N notifications" badge from the start.
func seedNotifications(db *gorm.DB, users []models.User) {
	fmt.Println("\nSeeding notifications...")

	if len(users) < 2 {
		fmt.Println("  - Need at least 2 users, skipping")
		return
	}

	created := 0
	for i, user := range users {
		// Each user gets a notification from the previous user (cyclic)
		actor := users[(i+len(users)-1)%len(users)]

		var existing models.Notification
		if err := db.Where("user_id = ? AND actor_id = ? AND type = ?",
			user.ID, actor.ID, "friend_request").First(&existing).Error; err == nil {
			continue
		}

		db.Create(&models.Notification{
			ID:            uuid.NewString(),
			CreatedAt:     time.Now().Add(-time.Hour),
			UserID:        user.ID,
			UserUsername:  user.Username,
			ActorID:       actor.ID,
			ActorUsername: actor.Username,
			Type:          "friend_request",
			Content:       actor.Username + " sent you a friend request",
			Read:          false,
		})
		created++
	}

	fmt.Printf("  + Created %d notifications\n", created)
}

// --- Existence checks ---

// friendExists checks if any relationship (any status) exists between two users
// in the given direction (user_id → friend_id).
func friendExists(db *gorm.DB, userID, friendID string) bool {
	var count int64
	db.Model(&models.Friend{}).
		Where("user_id = ? AND friend_id = ?", userID, friendID).
		Count(&count)
	return count > 0
}

// friendExistsWithStatus checks for a specific status.
func friendExistsWithStatus(db *gorm.DB, userID, friendID, status string) bool {
	var count int64
	db.Model(&models.Friend{}).
		Where("user_id = ? AND friend_id = ? AND status = ?", userID, friendID, status).
		Count(&count)
	return count > 0
}

// --- Main ---

func main() {
	db, err := config.ConnectDB()
	if err != nil {
		panic(err)
	}

	if err := ensureSchema(db); err != nil {
		panic(fmt.Errorf("schema migration failed: %w", err))
	}

	users := seedUsers(db)
	seedFriendships(db, users)
	seedFollows(db, users)
	posts := seedPosts(db, users)
	seedLikes(db, users, posts)
	seedReplies(db, users, posts)
	seedNotifications(db, users)

	fmt.Println("\nSeeding finished!")
}
