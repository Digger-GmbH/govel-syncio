package server

import (
	"context"

	"github.com/Digger-GmbH/govel-syncio/proto"
	"github.com/Digger-GmbH/govel-syncio/src/syncio"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	proto.UnimplementedSyncioServiceServer
	Store  *syncio.Store
	Engine *syncio.Engine
}

func NewGRPCServer(store *syncio.Store, engine *syncio.Engine) *GRPCServer {
	return &GRPCServer{Store: store, Engine: engine}
}

func toProtoConn(c syncio.Connection) *proto.Connection {
	return &proto.Connection{
		Id:                    c.ID,
		Name:                  c.Name,
		SourceShopId:          c.SourceShopID,
		DestinationShopIds:    c.DestinationShopIDs,
		Enabled:               c.Enabled,
		InventorySyncEnabled:  c.InventorySyncEnabled,
		ProductSyncEnabled:    c.ProductSyncEnabled,
		InventoryDirection:    c.InventoryDirection,
		MatchByBarcode:        c.MatchByBarcode,
		CreateMissingProducts: c.CreateMissingProducts,
		AttributeToggles:      c.AttributeToggles,
	}
}

func (s *GRPCServer) ListConnections(ctx context.Context, _ *proto.ListConnectionsRequest) (*proto.ListConnectionsResponse, error) {
	list := s.Store.List()
	out := make([]*proto.Connection, 0, len(list))
	for _, c := range list {
		out = append(out, toProtoConn(c))
	}
	return &proto.ListConnectionsResponse{Connections: out}, nil
}

func (s *GRPCServer) GetConnection(ctx context.Context, req *proto.GetConnectionRequest) (*proto.Connection, error) {
	c, ok := s.Store.Get(req.GetId())
	if !ok {
		return nil, status.Error(codes.NotFound, "connection not found")
	}
	return toProtoConn(c), nil
}

func (s *GRPCServer) UpsertConnection(ctx context.Context, req *proto.UpsertConnectionRequest) (*proto.Connection, error) {
	in := syncio.Connection{
		ID:                    req.GetId(),
		Name:                  req.GetName(),
		SourceShopID:          req.GetSourceShopId(),
		DestinationShopIDs:    req.GetDestinationShopIds(),
		InventoryDirection:    req.GetInventoryDirection(),
		AttributeToggles:      req.GetAttributeToggles(),
		Enabled:               true,
		InventorySyncEnabled:  true,
		MatchByBarcode:        true,
	}
	if req.Enabled != nil {
		in.Enabled = req.GetEnabled()
	}
	if req.InventorySyncEnabled != nil {
		in.InventorySyncEnabled = req.GetInventorySyncEnabled()
	}
	if req.ProductSyncEnabled != nil {
		in.ProductSyncEnabled = req.GetProductSyncEnabled()
	}
	if req.MatchByBarcode != nil {
		in.MatchByBarcode = req.GetMatchByBarcode()
	}
	if req.CreateMissingProducts != nil {
		in.CreateMissingProducts = req.GetCreateMissingProducts()
	}
	saved, err := s.Store.Upsert(in)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return toProtoConn(saved), nil
}

func (s *GRPCServer) DeleteConnection(ctx context.Context, req *proto.DeleteConnectionRequest) (*proto.DeleteConnectionResponse, error) {
	ok := s.Store.Delete(req.GetId())
	return &proto.DeleteConnectionResponse{Ok: ok}, nil
}

func (s *GRPCServer) ListAttributeToggles(ctx context.Context, req *proto.ListAttributeTogglesRequest) (*proto.ListAttributeTogglesResponse, error) {
	c, ok := s.Store.Get(req.GetConnectionId())
	if !ok {
		return nil, status.Error(codes.NotFound, "connection not found")
	}
	return &proto.ListAttributeTogglesResponse{ConnectionId: c.ID, Toggles: c.AttributeToggles}, nil
}

func (s *GRPCServer) SetAttributeToggles(ctx context.Context, req *proto.SetAttributeTogglesRequest) (*proto.ListAttributeTogglesResponse, error) {
	c, ok := s.Store.Get(req.GetConnectionId())
	if !ok {
		return nil, status.Error(codes.NotFound, "connection not found")
	}
	c.AttributeToggles = req.GetToggles()
	saved, err := s.Store.Upsert(c)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &proto.ListAttributeTogglesResponse{ConnectionId: saved.ID, Toggles: saved.AttributeToggles}, nil
}

func (s *GRPCServer) ListDefaultAttributes(ctx context.Context, _ *proto.ListDefaultAttributesRequest) (*proto.ListDefaultAttributesResponse, error) {
	return &proto.ListDefaultAttributesResponse{Attributes: syncio.DefaultProductAttributes}, nil
}

func (s *GRPCServer) ListProductMappings(ctx context.Context, req *proto.ListProductMappingsRequest) (*proto.ListProductMappingsResponse, error) {
	list := s.Store.ListMappings(req.GetConnectionId(), req.GetDestinationShopId())
	out := make([]*proto.ProductMapping, 0, len(list))
	for _, m := range list {
		pm := &proto.ProductMapping{
			Id: m.ID, ConnectionId: m.ConnectionID, DestinationShopId: m.DestinationShopID,
			MatchKey: m.MatchKey, MatchKind: m.MatchKind,
			SourceProductId: m.SourceProductID, SourceVariantId: m.SourceVariantID,
			DestProductId: m.DestProductID, DestVariantId: m.DestVariantID,
		}
		if m.LastSyncedAt != nil {
			pm.LastSyncedAt = m.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, pm)
	}
	return &proto.ListProductMappingsResponse{Mappings: out}, nil
}

func (s *GRPCServer) MapProductsBySku(ctx context.Context, req *proto.MapProductsBySkuRequest) (*proto.MapProductsBySkuResponse, error) {
	res, err := s.Engine.MapProductsBySku(ctx, req.GetConnectionId(), req.GetDestinationShopId(), req.GetDryRun())
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	out := make([]*proto.ProductMapping, 0, len(res.Mappings))
	for _, m := range res.Mappings {
		out = append(out, &proto.ProductMapping{
			Id: m.ID, ConnectionId: m.ConnectionID, DestinationShopId: m.DestinationShopID,
			MatchKey: m.MatchKey, MatchKind: m.MatchKind,
			SourceProductId: m.SourceProductID, SourceVariantId: m.SourceVariantID,
			DestProductId: m.DestProductID, DestVariantId: m.DestVariantID,
		})
	}
	return &proto.MapProductsBySkuResponse{
		Linked: res.Linked, UnmatchedSource: res.UnmatchedSource, UnmatchedDest: res.UnmatchedDest,
		Mappings: out, Notes: res.Notes,
	}, nil
}

func (s *GRPCServer) SyncInventory(ctx context.Context, req *proto.SyncInventoryRequest) (*proto.SyncInventoryResponse, error) {
	res, err := s.Engine.SyncInventory(ctx, req.GetConnectionId(), req.GetDryRun())
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	out := make([]*proto.InventoryDelta, 0, len(res.Deltas))
	for _, d := range res.Deltas {
		out = append(out, &proto.InventoryDelta{
			MatchKey: d.MatchKey, DestinationShopId: d.DestinationShopID,
			SourceVariantId: d.SourceVariantID, DestVariantId: d.DestVariantID,
			SourceQty: d.SourceQty, DestQty: d.DestQty, PlannedDestQty: d.PlannedDestQty,
			Action: d.Action, Reason: d.Reason,
		})
	}
	return &proto.SyncInventoryResponse{
		Planned: res.Planned, Applied: res.Applied, Skipped: res.Skipped, Blocked: res.Blocked,
		Deltas: out, Notes: res.Notes,
	}, nil
}

func (s *GRPCServer) SyncProducts(ctx context.Context, req *proto.SyncProductsRequest) (*proto.SyncProductsResponse, error) {
	res, err := s.Engine.SyncProducts(ctx, req.GetConnectionId(), req.GetDryRun())
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	out := make([]*proto.ProductSyncAction, 0, len(res.Actions))
	for _, a := range res.Actions {
		out = append(out, &proto.ProductSyncAction{
			MatchKey: a.MatchKey, DestinationShopId: a.DestinationShopID,
			Action: a.Action, Attributes: a.Attributes, Reason: a.Reason,
		})
	}
	return &proto.SyncProductsResponse{
		Planned: res.Planned, Applied: res.Applied, Skipped: res.Skipped, Blocked: res.Blocked,
		Actions: out, Notes: res.Notes,
	}, nil
}
