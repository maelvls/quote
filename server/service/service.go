package service

import (
	"strings"

	"github.com/sirupsen/logrus"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/maelvls/quote/schema/user"
	pb "github.com/maelvls/quote/schema/user"
	"github.com/rs/xid"
	context "golang.org/x/net/context"
)

// MemDB is a simple in-memory DB by Hashicorp. As I wanted to keep things
// simple, I did not go with Postgres.

// NewDB initializes the DB.
func NewDB() *memdb.MemDB {
	// Create the DB schema
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"user": {
				Name: "user",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {Name: "id", Unique: true,
						Indexer: &memdb.StringFieldIndex{Field: "Email"},
					},
					"age": {Name: "age", Unique: false,
						Indexer: &memdb.IntFieldIndex{Field: "Age"},
					},
				},
			},
		},
	}
	// Create a new data base
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		panic(err)
	}
	return db
}

// UserImpl implements my quote service. If I also wanted to be able to trace my
// service (e.g. using jaeger), I would also make sure to store
// opentracing.Tracer there.
type UserImpl struct {
	DB *memdb.MemDB
}

// NewUserImpl returns a new server.
func NewUserImpl() *UserImpl {
	return &UserImpl{DB: NewDB()}
}

// Create a user. If the given user has no id, generate one.
func (svc *UserImpl) Create(ctx context.Context, req *pb.CreateReq) (*pb.CreateResp, error) {
	user := req.GetUser()
	if user.Id == "" {
		user.Id = xid.New().String()
	}
	txn := svc.DB.Txn(true)
	if err := txn.Insert("user", user); err != nil {
		panic(err)
	}
	txn.Commit()

	resp := &pb.CreateResp{Status: &pb.Status{Code: pb.Status_SUCCESS}}
	return resp, nil
}

// List all users.
func (svc *UserImpl) List(ctx context.Context, req *pb.ListReq) (*pb.SearchResp, error) {
	// List all the people.
	txn := svc.DB.Txn(false) // read-only transaction
	defer txn.Abort()
	it, err := txn.Get("user", "id")
	if err != nil {
		panic(err)
	}

	var users = make([]*pb.User, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		user := raw.(*user.User)
		users = append(users, user)
	}

	resp := &pb.SearchResp{Users: users, Status: &pb.Status{Code: pb.Status_SUCCESS}}
	return resp, nil
}

// SearchAge searches all users in the range [from, to_included].
func (svc *UserImpl) SearchAge(ctx context.Context, req *pb.SearchAgeReq) (*pb.SearchResp, error) {
	if req.AgeRange == nil {
		return &pb.SearchResp{Users: make([]*pb.User, 0), Status: &pb.Status{
			Code: pb.Status_INVALID_QUERY,
			Msg:  "field AgeRange{From: int, ToIncluded: int} missing"},
		}, nil
	}
	if req.AgeRange.From > req.AgeRange.ToIncluded {
		return &pb.SearchResp{Users: make([]*pb.User, 0), Status: &pb.Status{
			Code: pb.Status_INVALID_QUERY,
			Msg:  "the From field must be lower or equal to ToIncluded"},
		}, nil
	}

	txn := svc.DB.Txn(false) // read-only transaction
	defer txn.Abort()

	// Range scan over people with ages in a range
	it, err := txn.LowerBound("user", "age", req.AgeRange.From)
	if err != nil {
		panic(err)
	}

	var users = make([]*pb.User, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		u := raw.(*user.User)
		if u.Age > 35 {
			break
		}
		users = append(users, u)
	}

	resp := &pb.SearchResp{Users: users, Status: &pb.Status{Code: pb.Status_SUCCESS}}
	return resp, nil
}

// SearchName searches a user by a part of its first or last name.
func (svc *UserImpl) SearchName(ctx context.Context, req *pb.SearchNameReq) (*pb.SearchResp, error) {
	if req.Query == "" {
		return &pb.SearchResp{Users: make([]*pb.User, 0), Status: &pb.Status{
			Code: pb.Status_INVALID_QUERY,
			Msg:  "query cannot be empty"},
		}, nil
	}
	filterByFirstOrLastName := func(query string) func(interface{}) bool {
		return func(raw interface{}) bool {
			u, ok := raw.(*pb.User)
			if !ok {
				logrus.Fatalf("could not unpack a quote.User, instead got: %#+v", raw)
			}

			return !(strings.Contains(u.Name.First, query) &&
				strings.Contains(u.Name.First, query))
		}
	}

	txn := svc.DB.Txn(false)
	defer txn.Abort()
	result, err := txn.Get("user", "id")
	if err != nil {
		logrus.Fatalf("err when getting data from db: %e\n", err)
	}

	it := memdb.NewFilterIterator(result, filterByFirstOrLastName(req.Query))

	var users = make([]*pb.User, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		u := raw.(*user.User)
		users = append(users, u)
	}

	resp := &pb.SearchResp{Users: users, Status: &pb.Status{Code: pb.Status_SUCCESS}}
	return resp, nil
}
