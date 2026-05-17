package store

import (
    "context"
    "errors"

    "github.com/google/uuid"
)

var (
    ErrFriendNotFound = errors.New("friend not found")
    ErrAlreadyFriends = errors.New("already friends")
    ErrFriendSelf     = errors.New("cannot add yourself as friend")
)

type CelebrityTransition struct {
    UserID  uuid.UUID
    IsCeleb bool
    Changed bool // true если is_celebrity изменилось в этой операции
}

type FriendStore struct {
    master    DB
    replica   DB
    threshold int
}

func NewFriendStore(master, replica DB) *FriendStore {
    return &FriendStore{master: master, replica: replica, threshold: 10000}
}

func (s *FriendStore) SetThreshold(threshold int) {
    s.threshold = threshold
}

func (s *FriendStore) AddFriend(ctx context.Context, userID, friendID uuid.UUID) (*CelebrityTransition, error) {
    if userID == friendID {
        return nil, ErrFriendSelf
    }

    tx, err := s.master.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)

    _, err = tx.Exec(ctx,
        "INSERT INTO friendship (user_id, friend_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
        userID, friendID,
    )
    if err != nil {
        return nil, mapPgError(err, ErrAlreadyFriends)
    }

    var newCount int
    var becameCeleb bool
    err = tx.QueryRow(ctx, `
        UPDATE users
        SET followers_count = followers_count + 1,
            is_celebrity = (followers_count + 1) >= $2
        WHERE id = $1
        RETURNING followers_count,
                  (followers_count >= $2) AND (followers_count - 1 < $2) AS became
    `, friendID, s.threshold).Scan(&newCount, &becameCeleb)
    if err != nil {
        return nil, err
    }

    if err := tx.Commit(ctx); err != nil {
        return nil, err
    }

    return &CelebrityTransition{
        UserID:  friendID,
        IsCeleb: newCount >= s.threshold,
        Changed: becameCeleb,
    }, nil
}

func (s *FriendStore) DeleteFriend(ctx context.Context, userID, friendID uuid.UUID) (*CelebrityTransition, error) {
    if userID == friendID {
        return nil, ErrFriendSelf
    }

    tx, err := s.master.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback(ctx)

    tag, err := tx.Exec(ctx,
        "DELETE FROM friendship WHERE user_id=$1 AND friend_id=$2",
        userID, friendID,
    )
    if err != nil {
        return nil, err
    }
    if tag.RowsAffected() == 0 {
        return nil, ErrFriendNotFound
    }

    var newCount int
    var lostCeleb bool
    err = tx.QueryRow(ctx, `
        UPDATE users
        SET followers_count = GREATEST(followers_count - 1, 0),
            is_celebrity = GREATEST(followers_count - 1, 0) >= $2
        WHERE id = $1
        RETURNING followers_count,
                  (followers_count < $2) AND (followers_count + 1 >= $2) AS lost
    `, friendID, s.threshold).Scan(&newCount, &lostCeleb)
    if err != nil {
        return nil, err
    }

    if err := tx.Commit(ctx); err != nil {
        return nil, err
    }

    return &CelebrityTransition{
        UserID:  friendID,
        IsCeleb: newCount >= s.threshold,
        Changed: lostCeleb,
    }, nil
}

func (s *FriendStore) GetFriendIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    rows, err := s.replica.Query(ctx,
        "SELECT friend_id FROM friendship WHERE user_id=$1",
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

// GetCelebrityFriendIDs returns friend IDs of userID that are celebrities.
func (s *FriendStore) GetCelebrityFriendIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    rows, err := s.replica.Query(ctx, `
        SELECT f.friend_id
        FROM friendship f
        JOIN users u ON u.id = f.friend_id
        WHERE f.user_id = $1 AND u.is_celebrity = true
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

// GetNonCelebrityFriendIDs returns friend IDs of userID that are NOT celebrities.
func (s *FriendStore) GetNonCelebrityFriendIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    rows, err := s.replica.Query(ctx, `
        SELECT f.friend_id
        FROM friendship f
        JOIN users u ON u.id = f.friend_id
        WHERE f.user_id = $1 AND u.is_celebrity = false
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

// IsCelebrity checks the persisted flag.
func (s *FriendStore) IsCelebrity(ctx context.Context, userID uuid.UUID) (bool, error) {
    var isCeleb bool
    err := s.replica.QueryRow(ctx,
        "SELECT is_celebrity FROM users WHERE id=$1", userID).Scan(&isCeleb)
    if err != nil {
        return false, err
    }
    return isCeleb, nil
}

// GetFollowers returns user IDs that have the given userID as a friend
// (i.e. rows where friend_id = userID).
func (s *FriendStore) GetFollowers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    rows, err := s.replica.Query(ctx,
        "SELECT user_id FROM friendship WHERE friend_id=$1",
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

// CountFollowers returns the number of followers for a user.
func (s *FriendStore) CountFollowers(ctx context.Context, userID uuid.UUID) (int, error) {
    var n int
    err := s.replica.QueryRow(ctx,
        "SELECT COUNT(*) FROM friendship WHERE friend_id=$1", userID).Scan(&n)
    return n, err
}

// RecalcCelebrityFlags recomputes followers_count and is_celebrity for all users.
func (s *FriendStore) RecalcCelebrityFlags(ctx context.Context) error {
    _, err := s.master.Exec(ctx, "SELECT recalc_celebrity_flags($1)", s.threshold)
    return err
}
