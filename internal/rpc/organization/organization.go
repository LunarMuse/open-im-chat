package organization

import (
	"context"
	"errors"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/discoveryregistry"
	"github.com/OpenIMSDK/Open-IM-Server/pkg/errs"
	"github.com/OpenIMSDK/chat/pkg/common/db/database"
	table "github.com/OpenIMSDK/chat/pkg/common/db/table/organization"
	"github.com/OpenIMSDK/chat/pkg/common/dbconn"
	"github.com/OpenIMSDK/chat/pkg/proto/common"
	"github.com/OpenIMSDK/chat/pkg/proto/organization"
	"github.com/OpenIMSDK/chat/pkg/rpclient/openim"
	organizationClient "github.com/OpenIMSDK/chat/pkg/rpclient/organization"
	"github.com/OpenIMSDK/open_utils/constant"
	"google.golang.org/grpc"
	"gorm.io/gorm"
	"strconv"
	"time"
)

func Start(discov discoveryregistry.SvcDiscoveryRegistry, server *grpc.Server) error {
	db, err := dbconn.NewGormDB()
	if err != nil {
		return err
	}
	//todo:修改
	tables := []any{
		chat2.Account{},
		chat2.Register{},
		chat2.Attribute{},
		chat2.VerifyCode{},
		chat2.UserLoginRecord{},
	}
	if err := db.AutoMigrate(tables...); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	organization.RegisterOrganizationServer(server, &organizationSvr{
		Database:     database.NewOrganizationDatabase(db),
		Organization: organizationClient.NewOrgClient(discov),
		OpenIM:       openim.NewOpenIMClient(discov),
	})
	return nil
}

type organizationSvr struct {
	Database     database.OrganizationDatabaseInterface
	Organization *organizationClient.OrgClient
	OpenIM       *openim.OpenIMClient
}

func (o *organizationSvr) CreateDepartment(ctx context.Context, req *organization.CreateDepartmentReq) (*organization.CreateDepartmentResp, error) {
	resp := &organization.CreateDepartmentResp{CommonResp: &common.CommonResp{}, DepartmentInfo: &common.Department{}}
	if req.DepartmentInfo == nil {
		resp.CommonResp.ErrCode = constant.ErrArgs.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrArgs.ErrMsg + " req.DepartmentInfo is nil"
		return resp, nil
	}
	department := table.Department{
		DepartmentID:   genDepartmentID(),
		FaceURL:        req.DepartmentInfo.FaceURL,
		Name:           req.DepartmentInfo.Name,
		ParentID:       req.DepartmentInfo.ParentID,
		Order:          req.DepartmentInfo.Order,
		DepartmentType: req.DepartmentInfo.DepartmentType,
		RelatedGroupID: req.DepartmentInfo.RelatedGroupID,
		CreateTime:     time.UnixMilli(req.DepartmentInfo.CreateTime),
	}
	if department.DepartmentID == "" {
		department.DepartmentID = strconv.FormatInt(time.Now().Unix(), 10)
	}
	if req.DepartmentInfo.ParentID != "" {
		_, err := o.Database.GetDepartmentByID(ctx, req.DepartmentInfo.ParentID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrArgs.Wrap("parent department not found")
		} else if err != nil {
			return nil, err
		}
	}
	if err := o.Database.CreateDepartment(ctx, &department); err != nil {
		return nil, err
	}
	return resp, nil
}

func (o *organizationSvr) UpdateDepartment(ctx context.Context, req *organization.UpdateDepartmentReq) (*organization.UpdateDepartmentResp, error) {
	resp := &organization.UpdateDepartmentResp{CommonResp: &common.CommonResp{}}

	if req.DepartmentInfo == nil {
		resp.CommonResp.ErrCode = constant.ErrArgs.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrArgs.ErrMsg + " req.DepartmentInfo is nil"
		return resp, nil
	}
	err := o.Database.UpdateDepartment(ctx, &table.Department{
		DepartmentID:   req.DepartmentInfo.DepartmentID,
		FaceURL:        req.DepartmentInfo.FaceURL,
		Name:           req.DepartmentInfo.Name,
		ParentID:       req.DepartmentInfo.ParentID,
		Order:          req.DepartmentInfo.Order,
		DepartmentType: req.DepartmentInfo.DepartmentType,
		RelatedGroupID: req.DepartmentInfo.RelatedGroupID,
	})
	if req.DepartmentInfo.ParentID != "" {
		_, err := o.Database.GetDepartmentByID(ctx, req.DepartmentInfo.ParentID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.CommonResp.ErrCode = constant.RecordNotFound
			resp.CommonResp.ErrMsg = "parent department not found"
			return resp, nil
		} else if err != nil {
			resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
			resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
			return resp, nil
		}
	}
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
	}

	return resp, nil
}

func (o *organizationSvr) GetOrganizationDepartment(ctx context.Context, req *organization.GetOrganizationDepartmentReq) (*organization.GetOrganizationDepartmentResp, error) {
	resp := &organization.GetOrganizationDepartmentResp{CommonResp: &common.CommonResp{}, DepartmentList: []*organization.DepartmentInfo{}}

	numMap, err := o.GetDepartmentMemberNum(ctx, "")
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
		return resp, nil
	}
	var getSubDepartmentList func(departmentId string, list *[]*organization.DepartmentInfo) error
	getSubDepartmentList = func(departmentId string, list *[]*organization.DepartmentInfo) error {
		departments, err := o.Database.GetParent(ctx, departmentId)
		if err != nil {
			return err
		}
		for _, department := range departments {
			subs := make([]*organization.DepartmentInfo, 0)
			err = getSubDepartmentList(department.DepartmentID, &subs)
			if err != nil {
				return err
			}
			*list = append(*list, &organization.DepartmentInfo{
				Department: &common.Department{
					DepartmentID:   department.DepartmentID,
					FaceURL:        department.FaceURL,
					Name:           department.Name,
					ParentID:       department.ParentID,
					Order:          department.Order,
					DepartmentType: department.DepartmentType,
					RelatedGroupID: department.RelatedGroupID,
					CreateTime:     department.CreateTime.UnixMilli(),
					MemberNum:      uint32(numMap[department.DepartmentID]),
				},
				SubDepartmentList: subs,
			})
		}
		return nil
	}

	if err := getSubDepartmentList("", &resp.DepartmentList); err != nil {
		return nil, err
	}
	return resp, nil
}

func (o *organizationSvr) DeleteDepartment(ctx context.Context, req *organization.DeleteDepartmentReq) (*organization.DeleteDepartmentResp, error) {
	resp := &organization.DeleteDepartmentResp{CommonResp: &common.CommonResp{}}
	departmentList, err := o.Database.GetList(ctx, req.DepartmentIDList)
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
		return resp, nil
	}
	if len(departmentList) == 0 {
		return resp, nil
	}
	// 修改删除的子部门的父部门为删除的上级
	for _, department := range departmentList {
		err := o.Database.UpdateParentID(ctx, department.DepartmentID, department.ParentID)
		if err != nil {
			resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
			resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + "update parent_id " + err.Error()
			return resp, nil
		}
	}
	// 删除部门
	if err := o.Database.Delete(ctx, req.DepartmentIDList); err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
		return resp, nil
	}
	// 删除职位信息
	if err := o.Database.DeleteDepartmentIDList(ctx, req.DepartmentIDList); err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
		return resp, nil
	}
	return resp, nil
}

func (o *organizationSvr) GetDepartment(ctx context.Context, req *organization.GetDepartmentReq) (*organization.GetDepartmentResp, error) {
	

}

func (o *organizationSvr) CreateOrganizationUser(ctx context.Context, req *organization.CreateOrganizationUserReq) (*organization.CreateOrganizationUserResp, error) {
	resp := &organization.CreateOrganizationUserResp{CommonResp: &common.CommonResp{}}
	if req.OrganizationUser == nil {
		resp.CommonResp.ErrCode = constant.ErrArgs.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrArgs.ErrMsg + " req.OrganizationUser is nil"
		return resp, nil
	}
	err := o.Database.CreateOrganizationUser(ctx, &table.OrganizationUser{
		UserID:      req.OrganizationUser.UserID,
		Nickname:    req.OrganizationUser.Nickname,
		EnglishName: req.OrganizationUser.EnglishName,
		FaceURL:     req.OrganizationUser.FaceURL,
		Gender:      req.OrganizationUser.Gender,
		Mobile:      req.OrganizationUser.Mobile,
		Telephone:   req.OrganizationUser.Telephone,
		Birth:       time.UnixMilli(req.OrganizationUser.Birth),
		Email:       req.OrganizationUser.Email,
		Status:      req.OrganizationUser.Status,
		Station:     req.OrganizationUser.Station,
		AreaCode:    req.OrganizationUser.AreaCode,
	})
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
	}
	return resp, nil
}

func (o *organizationSvr) UpdateOrganizationUser(ctx context.Context, req *organization.UpdateOrganizationUserReq) (*organization.UpdateOrganizationUserResp, error) {
	resp := &organization.UpdateOrganizationUserResp{CommonResp: &common.CommonResp{}}
	if req.OrganizationUser == nil {
		resp.CommonResp.ErrCode = constant.ErrArgs.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrArgs.ErrMsg + " req.OrganizationUser is nil"
		return resp, nil
	}
	err := o.Database.UpdateOrganizationUser(ctx, &table.OrganizationUser{
		UserID:      req.OrganizationUser.UserID,
		Nickname:    req.OrganizationUser.Nickname,
		EnglishName: req.OrganizationUser.EnglishName,
		FaceURL:     req.OrganizationUser.FaceURL,
		Gender:      req.OrganizationUser.Gender,
		Mobile:      req.OrganizationUser.Mobile,
		Telephone:   req.OrganizationUser.Telephone,
		Birth:       time.UnixMilli(req.OrganizationUser.Birth),
		Email:       req.OrganizationUser.Email,
		Status:      req.OrganizationUser.Status,
		Station:     req.OrganizationUser.Station,
		AreaCode:    req.OrganizationUser.AreaCode,
	})
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
	}
	return resp, nil
}

func (o *organizationSvr) DeleteOrganizationUser(ctx context.Context, req *organization.DeleteOrganizationUserReq) (*organization.DeleteOrganizationUserResp, error) {
	resp := &organization.DeleteOrganizationUserResp{CommonResp: &common.CommonResp{}}
	err := o.Database.DeleteOrganizationUser(ctx, req.UserID)
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
		return resp, nil
	}
	err = o.Database.DeleteDepartmentMember(ctx, req.UserID)
	if err != nil {
		resp.CommonResp.ErrCode = constant.ErrDB.ErrCode
		resp.CommonResp.ErrMsg = constant.ErrDB.ErrMsg + err.Error()
	}
	return resp, nil
}

func (o *organizationSvr) CreateDepartmentMember(ctx context.Context, req *organization.CreateDepartmentMemberReq) (*organization.CreateDepartmentMemberResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetUserInDepartment(ctx context.Context, req *organization.GetUserInDepartmentReq) (*organization.GetUserInDepartmentResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) DeleteUserInDepartment(ctx context.Context, req *organization.DeleteUserInDepartmentReq) (*organization.DeleteUserInDepartmentResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) UpdateUserInDepartment(ctx context.Context, req *organization.UpdateUserInDepartmentReq) (*organization.UpdateUserInDepartmentResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetSearchUserList(ctx context.Context, req *organization.GetSearchUserListReq) (*organization.GetSearchUserListResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) SetOrganization(ctx context.Context, req *organization.SetOrganizationReq) (*organization.SetOrganizationResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetOrganization(ctx context.Context, req *organization.GetOrganizationReq) (*organization.GetOrganizationResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetSubDepartment(ctx context.Context, req *organization.GetSubDepartmentReq) (*organization.GetSubDepartmentResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetSearchDepartmentUser(ctx context.Context, req *organization.GetSearchDepartmentUserReq) (*organization.GetSearchDepartmentUserResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) SortDepartmentList(ctx context.Context, req *organization.SortDepartmentListReq) (*organization.SortDepartmentListResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) SortOrganizationUserList(ctx context.Context, req *organization.SortOrganizationUserListReq) (*organization.SortOrganizationUserListResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) CreateNewOrganizationMember(ctx context.Context, req *organization.CreateNewOrganizationMemberReq) (*organization.CreateNewOrganizationMemberResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetUserInfo(ctx context.Context, req *organization.GetUserInfoReq) (*organization.GetUserInfoResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) BatchImport(ctx context.Context, req *organization.BatchImportReq) (*organization.BatchImportResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) MoveUserDepartment(ctx context.Context, req *organization.MoveUserDepartmentReq) (*organization.MoveUserDepartmentResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) GetUserFullList(ctx context.Context, req *organization.GetUserFullListReq) (*organization.GetUserFullListResp, error) {
	//TODO implement me
	panic("implement me")
}

func (o *organizationSvr) SearchUsersFullInfo(ctx context.Context, req *organization.SearchUsersFullInfoReq) (*organization.SearchUsersFullInfoResp, error) {
	//TODO implement me
	panic("implement me")
}
