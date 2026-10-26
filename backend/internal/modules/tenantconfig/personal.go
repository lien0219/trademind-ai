package tenantconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PersonalPreferences struct {
	Language string `json:"language"`
	Theme string `json:"theme"`
	DateFormat string `json:"dateFormat"`
}

type PersonalSnapshot struct {
	Values PersonalPreferences `json:"values"`
	Version int64 `json:"version"`
	Inherited bool `json:"inherited"`
}

func DefaultPersonalPreferences() PersonalPreferences {
	return PersonalPreferences{Language:"zh-CN", Theme:"light", DateFormat:"YYYY-MM-DD"}
}

func ValidatePersonalPreferences(raw []byte) (PersonalPreferences,error) {
	var prefs PersonalPreferences
	if len(raw)==0 || len(raw)>512 || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {return prefs,errors.New("invalid personal preference size or shape")}
	decoder:=json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err:=decoder.Decode(&prefs);err!=nil{return prefs,err}
	if err:=decoder.Decode(new(any));err!=io.EOF{return prefs,errors.New("unexpected personal preference payload")}
	if !oneOf(prefs.Language,"zh-CN","en-US","ja-JP") || !oneOf(prefs.Theme,"light","dark") || !oneOf(prefs.DateFormat,"YYYY-MM-DD","DD/MM/YYYY","MM/DD/YYYY") {
		return prefs,errors.New("unsupported personal preferences")
	}
	return prefs,nil
}

func ReadPersonalPreferences(ctx context.Context,db *gorm.DB,tenantID int64,userID uuid.UUID)(PersonalSnapshot,error){
	if db==nil||tenantID<=0||userID==uuid.Nil{return PersonalSnapshot{},errors.New("invalid session")}
	var row UserPreference
	err:=db.WithContext(ctx).Where("tenant_id=? AND user_id=?",tenantID,userID).Take(&row).Error
	if errors.Is(err,gorm.ErrRecordNotFound){return PersonalSnapshot{Values:DefaultPersonalPreferences(),Inherited:true},nil}
	if err!=nil{return PersonalSnapshot{},err}
	prefs,err:=ValidatePersonalPreferences(row.Values)
	if err!=nil{return PersonalSnapshot{},err}
	return PersonalSnapshot{Values:prefs,Version:row.Version},nil
}

func WritePersonalPreferences(ctx context.Context,db *gorm.DB,tenantID int64,userID uuid.UUID,raw []byte,expected int64)(PersonalSnapshot,error){
	if db==nil||tenantID<=0||userID==uuid.Nil||expected<0{return PersonalSnapshot{},errors.New("invalid session")}
	prefs,err:=ValidatePersonalPreferences(raw)
	if err!=nil{return PersonalSnapshot{},err}
	normalized,err:=json.Marshal(prefs)
	if err!=nil{return PersonalSnapshot{},err}
	next:=expected+1
	err=db.WithContext(ctx).Transaction(func(tx *gorm.DB)error{
		var row UserPreference
		err:=tx.Where("tenant_id=? AND user_id=?",tenantID,userID).Take(&row).Error
		if errors.Is(err,gorm.ErrRecordNotFound){
			if expected!=0{return ErrVersionConflict}
			return tx.Create(&UserPreference{TenantID:tenantID,UserID:userID,Version:1,Values:datatypes.JSON(normalized)}).Error
		}
		if err!=nil{return err}
		if row.Version!=expected{return ErrVersionConflict}
		result:=tx.Model(&UserPreference{}).Where("tenant_id=? AND user_id=? AND version=?",tenantID,userID,expected).
			Updates(map[string]any{"version":next,"values":datatypes.JSON(normalized)})
		if result.Error!=nil{return result.Error}
		if result.RowsAffected!=1{return ErrVersionConflict}
		return nil
	})
	if err!=nil{return PersonalSnapshot{},err}
	return PersonalSnapshot{Values:prefs,Version:next},nil
}
