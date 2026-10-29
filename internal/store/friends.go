package store

import (
	"database/sql"
	"fmt"
)

type FriendList struct {
	Friends  []PublicUser `json:"friends"`
	Incoming []PublicUser `json:"incoming"`
	Outgoing []string     `json:"outgoing"`
}

func (db *DB) ListFriends(userID string, online map[string]bool) (FriendList, error) {
	friends, err := db.friendUsers(`
		SELECT u.id, u.nickname, u.avatar_path, u.signature, u.pendant, u.dnd
		FROM friends f
		JOIN users u ON u.id = f.friend_id
		WHERE f.user_id = ? AND u.banned = 0
		ORDER BY u.nickname, u.id`, userID, online)
	if err != nil {
		return FriendList{}, err
	}
	incoming, err := db.friendUsers(`
		SELECT u.id, u.nickname, u.avatar_path, u.signature, u.pendant, u.dnd
		FROM friend_requests r
		JOIN users u ON u.id = r.from_id
		WHERE r.to_id = ? AND u.banned = 0
		ORDER BY r.created_at, u.id`, userID, online)
	if err != nil {
		return FriendList{}, err
	}
	outgoing, err := db.outgoingIDs(userID)
	if err != nil {
		return FriendList{}, err
	}
	return FriendList{Friends: friends, Incoming: incoming, Outgoing: outgoing}, nil
}

func (db *DB) outgoingIDs(userID string) ([]string, error) {
	rows, err := db.sql.Query(`SELECT to_id FROM friend_requests WHERE from_id = ? ORDER BY created_at, to_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (db *DB) RequestFriend(fromID, toID string, nowMs int64) error {
	if fromID == toID {
		return fmt.Errorf("不能添加自己")
	}
	target, err := db.UserByID(toID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("用户不存在")
		}
		return err
	}
	if target.Banned {
		return fmt.Errorf("对方账号不可用")
	}
	friends, err := db.areFriends(fromID, toID)
	if err != nil {
		return err
	}
	if friends {
		return fmt.Errorf("已经是好友")
	}
	reverse, err := db.hasRequest(toID, fromID)
	if err != nil {
		return err
	}
	if reverse {
		return db.acceptFriend(fromID, toID, nowMs)
	}
	return db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(
			`INSERT INTO friend_requests (from_id, to_id, created_at) VALUES (?, ?, ?)
			 ON CONFLICT (from_id, to_id) DO NOTHING`,
			fromID, toID, nowMs,
		)
		return execErr
	})
}

func (db *DB) RemoveFriend(userID, otherID string) error {
	return db.withLockRetry(func() error {
		if _, err := db.sql.Exec(
			`DELETE FROM friends WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
			userID, otherID, otherID, userID,
		); err != nil {
			return err
		}
		_, err := db.sql.Exec(
			`DELETE FROM friend_requests WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)`,
			userID, otherID, otherID, userID,
		)
		return err
	})
}

func (db *DB) acceptFriend(userID, otherID string, nowMs int64) error {
	return db.withLockRetry(func() error {
		if _, err := db.sql.Exec(
			`INSERT INTO friends (user_id, friend_id, created_at) VALUES (?, ?, ?)
			 ON CONFLICT (user_id, friend_id) DO NOTHING`,
			userID, otherID, nowMs,
		); err != nil {
			return err
		}
		if _, err := db.sql.Exec(
			`INSERT INTO friends (user_id, friend_id, created_at) VALUES (?, ?, ?)
			 ON CONFLICT (user_id, friend_id) DO NOTHING`,
			otherID, userID, nowMs,
		); err != nil {
			return err
		}
		_, err := db.sql.Exec(
			`DELETE FROM friend_requests WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)`,
			userID, otherID, otherID, userID,
		)
		return err
	})
}

func (db *DB) areFriends(a, b string) (bool, error) {
	var n int
	err := db.sql.QueryRow(
		`SELECT COUNT(*) FROM friends WHERE user_id = ? AND friend_id = ?`,
		a, b,
	).Scan(&n)
	return n > 0, err
}

func (db *DB) hasRequest(fromID, toID string) (bool, error) {
	var n int
	err := db.sql.QueryRow(
		`SELECT COUNT(*) FROM friend_requests WHERE from_id = ? AND to_id = ?`,
		fromID, toID,
	).Scan(&n)
	return n > 0, err
}

func (db *DB) friendUsers(query, userID string, online map[string]bool) ([]PublicUser, error) {
	rows, err := db.sql.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicUser{}
	for rows.Next() {
		var id, nickname, avatar, signature, pendant string
		var quiet int
		if err := rows.Scan(&id, &nickname, &avatar, &signature, &pendant, &quiet); err != nil {
			return nil, err
		}
		pub := Public(User{
			ID: id, Nickname: nickname, AvatarPath: avatar, Signature: signature, Pendant: pendant, Dnd: quiet != 0,
		})
		pub.Online = online[id]
		out = append(out, pub)
	}
	return out, rows.Err()
}
