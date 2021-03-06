package controllers

import (
	"encoding/json"
	"github.com/astaxie/beego/context"
	"net/http"
	"strconv"

	"github.com/hzwy23/asofdate/hauth/hcache"
	"github.com/hzwy23/asofdate/hauth/models"

	"github.com/asaskevich/govalidator"
	"github.com/hzwy23/asofdate/hauth/hrpc"
	"github.com/hzwy23/asofdate/utils"
	"github.com/hzwy23/asofdate/utils/hret"
	"github.com/hzwy23/asofdate/utils/i18n"
	"github.com/hzwy23/asofdate/utils/logs"
	"github.com/hzwy23/asofdate/utils/token/hjwt"
	"github.com/tealeg/xlsx"
	"os"
	"path/filepath"
	"io/ioutil"
)

type orgController struct {
	models *models.OrgModel
	upload   chan int
}

var OrgCtl = &orgController{
	models: new(models.OrgModel),
	upload:make(chan int,1),
}

func (orgController) Page(ctx *context.Context) {
	ctx.Request.ParseForm()
	if !hrpc.BasicAuth(ctx) {
		return
	}
	rst, err := hcache.GetStaticFile("AsofdateOrgPage")
	if err != nil {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 404, "页面不存在")
		return
	}
	ctx.ResponseWriter.Write(rst)
}

// 获取机构信息
func (this orgController) Get(ctx *context.Context) {
	ctx.Request.ParseForm()
	if !hrpc.BasicAuth(ctx) {
		return
	}

	domain_id := ctx.Request.FormValue("domain_id")

	if domain_id == "" {
		cookie, _ := ctx.Request.Cookie("Authorization")
		jclaim, err := hjwt.ParseJwt(cookie.Value)
		if err != nil {
			logs.Error(err)
			hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, "No Auth")
			return
		}
		domain_id = jclaim.Domain_id
	}

	if !hrpc.CheckDomain(ctx, domain_id, "r") {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "没有权限访问这个域中的信息。")
		return
	}

	rst, err := this.models.Get(domain_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 417, "操作数据库失败")
		return
	}
	hret.WriteJson(ctx.ResponseWriter, rst)
}

func (this orgController) Delete(ctx *context.Context) {
	ctx.Request.ParseForm()
	if !hrpc.BasicAuth(ctx) {
		return
	}

	domain_id := ctx.Request.FormValue("domain_id")
	orgList := ctx.Request.FormValue("JSON")
	var mjs []models.SysOrgInfo
	err := json.Unmarshal([]byte(orgList), &mjs)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, http.StatusExpectationFailed, "delete org info failed.", err)
		return
	}

	if govalidator.IsEmpty(domain_id) {
		cok, _ := ctx.Request.Cookie("Authorization")
		jclaim, err := hjwt.ParseJwt(cok.Value)
		if err != nil {
			hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, i18n.Disconnect())
			return
		}
		domain_id = jclaim.Domain_id
	}

	if !hrpc.CheckDomain(ctx,domain_id,"w"){
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, "获取机构对应的域信息错误.")
		return
	}


	err = this.models.Delete(mjs,domain_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 418, err.Error())
		return
	}

	hret.WriteHttpOkMsgs(ctx.ResponseWriter, "delete org info successfully.")
}

func (this orgController) Update(ctx *context.Context) {
	ctx.Request.ParseForm()
	if !hrpc.BasicAuth(ctx) {
		return
	}

	cookie, _ := ctx.Request.Cookie("Authorization")
	jclaim, err := hjwt.ParseJwt(cookie.Value)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, "No Auth")
		return
	}
	org_unit_id := ctx.Request.FormValue("Id")
	org_unit_desc := ctx.Request.FormValue("Org_unit_desc")
	up_org_id := ctx.Request.FormValue("Up_org_id")
	org_status_id := ctx.Request.FormValue("Status_cd")

	did, err := hrpc.CheckDomainByOrgId(org_unit_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "您没有权限更新这个域中的机构信息")
		return
	}

	if !hrpc.CheckDomain(ctx, did, "w") {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "您没有权限更新这个域中的机构信息")
		return
	}

	// 校验输入信息
	if govalidator.IsEmpty(org_unit_desc) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "机构描述信息不能为空.")
		return
	}

	if !govalidator.IsIn(org_status_id, "0", "1") {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "请选择机构状态.")
		return
	}

	if !govalidator.IsWord(org_unit_id) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "机构编码不正确")
		return
	}

	if !govalidator.IsWord(up_org_id) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "请选择上级机构.")
		return
	}

	check, err := this.models.GetSubOrgInfo(did, org_unit_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "操作数据库失败。")
		return
	}

	for _, val := range check {
		if val.Org_unit_id == up_org_id {
			hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "上级机构号不能是自己的下属机构")
			return
		}
	}

	err = this.models.Update(org_unit_desc, up_org_id, org_status_id, jclaim.User_id, org_unit_id, did)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "修改机构信息失败", err)
		return
	}
	hret.WriteHttpOkMsgs(ctx.ResponseWriter, i18n.Get("success"))
}

func (this orgController) Post(ctx *context.Context) {
	ctx.Request.ParseForm()
	if !hrpc.BasicAuth(ctx) {
		return
	}
	cookie, _ := ctx.Request.Cookie("Authorization")
	jclaim, err := hjwt.ParseJwt(cookie.Value)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 310, "No Auth")
		return
	}

	org_unit_id := ctx.Request.FormValue("Org_unit_id")
	org_unit_desc := ctx.Request.FormValue("Org_unit_desc")
	up_org_id := ctx.Request.FormValue("Up_org_id")
	domain_id := ctx.Request.FormValue("Domain_id")

	if !hrpc.CheckDomain(ctx, domain_id, "w") {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "您没有权限在这个中新增机构信息")
		return
	}

	id := utils.JoinCode(domain_id, org_unit_id)
	org_status_id := "0"

	if !govalidator.IsWord(org_unit_id) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "机构编码必须有1,30位字母或数字组成")
		return
	}

	if govalidator.IsEmpty(org_unit_desc) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "机构名称不能为空，请输入机构名称")
		return
	}

	if !govalidator.IsWord(domain_id) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "请选择所属域，所属域不能为空")
		return
	}

	if !govalidator.IsWord(up_org_id) {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "请选择上级机构号，上级机构号不能为空")
		return
	}

	if !govalidator.IsIn(org_status_id, "0", "1") {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "机构状态不正确.")
		return
	}

	err = this.models.Post(org_unit_id, org_unit_desc, up_org_id, org_status_id,
		domain_id, jclaim.User_id, jclaim.User_id, id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "新增机构信息失败.", err)
		return
	}
	hret.WriteHttpOkMsgs(ctx.ResponseWriter, i18n.Get("success"))
}

func (orgController) getOrgTops(node []models.SysOrgInfo) []models.SysOrgInfo {
	var ret []models.SysOrgInfo
	for _, val := range node {
		flag := true
		for _, iv := range node {
			if val.Up_org_id == iv.Org_unit_id {
				flag = false
			}
		}
		if flag {
			ret = append(ret, val)
		}
	}
	return ret
}

func (this orgController) orgTree(node []models.SysOrgInfo, id string, d int, result *[]models.SysOrgInfo) {
	var oneline models.SysOrgInfo
	for _, val := range node {
		if val.Up_org_id == id {
			oneline = val
			oneline.Org_dept = strconv.Itoa(d)
			*result = append(*result, oneline)
			this.orgTree(node, val.Org_unit_id, d+1, result)
		}
	}
}

func (this orgController) GetSubOrgInfo(ctx *context.Context) {

	ctx.Request.ParseForm()

	org_unit_id := ctx.Request.FormValue("org_unit_id")

	did, err := hrpc.CheckDomainByOrgId(org_unit_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "您没有权限更新这个域中的机构信息")
		return
	}

	rst, err := this.models.GetSubOrgInfo(did, org_unit_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 419, "操作数据库失败")
		return
	}

	hret.WriteJson(ctx.ResponseWriter, rst)
}

func (this orgController) Download(ctx *context.Context) {
	ctx.Request.ParseForm()
	if !hrpc.BasicAuth(ctx) {
		return
	}

	ctx.ResponseWriter.Header().Set("Content-Type", "application/vnd.ms-excel")
	domain_id := ctx.Request.FormValue("domain_id")

	if govalidator.IsEmpty(domain_id) {
		cookie, _ := ctx.Request.Cookie("Authorization")
		jclaim, err := hjwt.ParseJwt(cookie.Value)
		if err != nil {
			logs.Error(err)
			hret.WriteHttpErrMsgs(ctx.ResponseWriter, 310, "No Auth")
			return
		}
		domain_id = jclaim.Domain_id
	}

	if !hrpc.CheckDomain(ctx, domain_id, "r") {
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, "您没有权限导出这个域中的机构信息.")
		return
	}

	rst, err := this.models.Get(domain_id)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 417, "操作数据库失败")
		return
	}

	var sheet *xlsx.Sheet
	HOME := os.Getenv("HBIGDATA_HOME")
	file, err := xlsx.OpenFile(filepath.Join(HOME, "upload/template/hauthOrgExportTemplate.xlsx"))
	if err != nil {
		file = xlsx.NewFile()
		sheet, err = file.AddSheet("机构信息")
		if err != nil {
			logs.Error(err)
			hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "没有找到sheet也名称为'机构信息'的页面")
			return
		}

		{
			row := sheet.AddRow()
			cell1 := row.AddCell()
			cell1.Value = "机构编码"
			cell2 := row.AddCell()
			cell2.Value = "机构名称"
			cell3 := row.AddCell()
			cell3.Value = "上级编码"
			cell9 := row.AddCell()
			cell9.Value = "所属域"
			cell4 := row.AddCell()
			cell4.Value = "机构状态"

			cell5 := row.AddCell()
			cell5.Value = "创建日期"
			cell6 := row.AddCell()
			cell6.Value = "创建人"
			cell7 := row.AddCell()
			cell7.Value = "维护日期"
			cell8 := row.AddCell()
			cell8.Value = "维护人"

		}
	} else {
		sheet = file.Sheet["机构信息"]
		if sheet == nil {
			hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "没有找到sheet也名称为'机构信息'的页面")
			return
		}
	}
	for _, v := range rst {
		row := sheet.AddRow()
		cell1 := row.AddCell()
		cell1.Value = v.Code_number
		cell1.SetStyle(sheet.Rows[1].Cells[0].GetStyle())

		cell2 := row.AddCell()
		cell2.Value = v.Org_unit_desc
		cell2.SetStyle(sheet.Rows[1].Cells[1].GetStyle())

		cell3 := row.AddCell()
		cell3.Value = utils.SplitCode(v.Up_org_id)
		cell3.SetStyle(sheet.Rows[1].Cells[2].GetStyle())

		cell9 := row.AddCell()
		cell9.Value = v.Domain_id
		cell9.SetStyle(sheet.Rows[1].Cells[3].GetStyle())

		cell4 := row.AddCell()
		cell4.Value = v.Org_status_desc
		cell4.SetStyle(sheet.Rows[1].Cells[4].GetStyle())

		cell5 := row.AddCell()
		cell5.Value = v.Create_date
		cell5.SetStyle(sheet.Rows[1].Cells[5].GetStyle())

		cell6 := row.AddCell()
		cell6.Value = v.Create_user
		cell6.SetStyle(sheet.Rows[1].Cells[6].GetStyle())

		cell7 := row.AddCell()
		cell7.Value = v.Maintance_date
		cell7.SetStyle(sheet.Rows[1].Cells[7].GetStyle())

		cell8 := row.AddCell()
		cell8.Value = v.Maintance_user
		cell8.SetStyle(sheet.Rows[1].Cells[8].GetStyle())

	}

	if len(sheet.Rows) >= 3 {
		sheet.Rows = append(sheet.Rows[0:1], sheet.Rows[2:]...)
	}

	file.Write(ctx.ResponseWriter)
}


func (this orgController)Upload(ctx *context.Context){
	if len(this.upload) != 0 {
		hret.WriteHttpOkMsgs(ctx.ResponseWriter, "已经有正在导入的任务,请稍等")
		return
	}

	// 从cookies中获取用户连接信息
	cookie, _ := ctx.Request.Cookie("Authorization")
	jclaim, err := hjwt.ParseJwt(cookie.Value)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, "No Auth")
		return
	}

	// 同一个时间,只能有一个导入任务
	this.upload <- 1
	defer func() {
		<-this.upload
	}()

	ctx.Request.ParseForm()
	fd, _, err := ctx.Request.FormFile("file")
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "读取上传文件失败")
		return
	}

	result,err := ioutil.ReadAll(fd)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter,421,"读取文件信息失败.")
		return
	}

	// 读取上传过来的文件信息
	// 转换成二进制数据流
	file, err := xlsx.OpenBinary(result)
	sheet, ok := file.Sheet["机构信息"]
	if !ok {
		logs.Error("没有找到'机构信息'这个sheet页")
		hret.WriteHttpErrMsgs(ctx.ResponseWriter, 421, "没有找到'机构信息'这个sheet页")
		return
	}
	var data []models.SysOrgInfo
	for index, val := range sheet.Rows {
		if index > 0 {
			var one models.SysOrgInfo
			one.Code_number = val.Cells[0].Value
			one.Org_unit_desc = val.Cells[1].Value
			one.Domain_id = val.Cells[3].Value
			one.Org_unit_id = utils.JoinCode(one.Domain_id,one.Code_number)
			one.Up_org_id = utils.JoinCode(one.Domain_id,val.Cells[2].Value)
			one.Create_user = jclaim.User_id

			if !hrpc.CheckDomain(ctx, one.Domain_id, "w") {
				hret.WriteHttpErrMsgs(ctx.ResponseWriter, 403, "您没有权限在"+val.Cells[6].Value+"域中导入机构信息")
				return
			}
			data = append(data,one)
		}
	}
	err = this.models.Upload(data)
	if err != nil {
		logs.Error(err)
		hret.WriteHttpErrMsgs(ctx.ResponseWriter,421,err.Error())
		return
	}

	hret.WriteHttpOkMsgs(ctx.ResponseWriter,i18n.Success())
}

func init() {
	hcache.Register("AsofdateOrgPage", "./views/hauth/org_page.tpl")
}
