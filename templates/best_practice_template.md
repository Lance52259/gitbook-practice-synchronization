# 部署{场景简述}

（H1 必须以「部署」开头，且不以「最佳实践」结尾。正确：`# 部署黑白名单`；错误：`# AAD黑白名单最佳实践`。）

## 应用场景

{结合华为云产品说明与本实践目标，用 2～4 句说明场景。第二段起句用「本最佳实践将介绍如何使用Terraform…」。勿保留元注释括号。}

## 相关资源/数据源

（无 data source 时导语改为「本最佳实践涉及以下主要资源：」，并删除下面的「### 数据源」整节。）

本最佳实践涉及以下主要资源和数据源：

### 数据源

- [{释义}（data.huaweicloud_xxx）](https://registry.terraform.io/providers/huaweicloud/huaweicloud/latest/docs/data-sources/xxx)

### 资源

- [{释义}（huaweicloud_xxx）](https://registry.terraform.io/providers/huaweicloud/huaweicloud/latest/docs/resources/xxx)

### 资源/数据源依赖关系

```
data.huaweicloud_xxx
    └── huaweicloud_xxx
```

## 操作步骤

### 1. 脚本准备

在指定工作空间中准备好用于编写当前最佳实践脚本的TF文件（如main.tf），确保其中（也可以是其他同级目录下的TF文件）包含部署资源所需的provider版本声明和华为云鉴权信息。
配置介绍参考[部署华为云资源前的准备工作](../../introductions/prepare_before_deploy.md)一文中的介绍。

### 2. {按脚本顺序说明第一个数据源或资源}

在TF文件（如main.tf）中添加以下脚本：

```hcl
# 未声明 region 时必须用完整缺省继承句：在指定region（region参数缺省时默认继承当前provider块中所指定的region）下…
# 仅在本步「首次引入」时声明 variable；若前序步骤已声明同名变量，此处禁止再写 variable 块，只引用 var.xxx
variable "example_name" {
  description = "…"
  type        = string
}

resource "huaweicloud_xxx" "test" {
  name = var.example_name
}
```

**参数说明**：
- **name**：通过引用输入变量 example_name 进行赋值

### n. 预设资源部署所需的入参（可选）

（标题中的「（可选）」不可省略。）

本实践中，部分资源、数据源使用了输入变量对配置内容进行赋值，这些输入参数在后续部署时需要手工输入。
同时，Terraform提供了通过`tfvars`文件预设这些配置的方法，可以避免每次执行时重复输入。

在工作目录下创建`terraform.tfvars`文件，示例内容如下：

```hcl
# 根据脚本变量填写；敏感信息使用占位符
vpc_name = "example-vpc"
```

**使用方法**：

1. 将上述内容保存为工作目录下的`terraform.tfvars`文件（该文件名可使用户在执行terraform命令时自动导入该`tfvars`文件中的内容，其他命名则需要在tfvars前补充`.auto`定义，如`variables.auto.tfvars`）
2. 根据实际需要修改参数值
3. 执行`terraform plan`或`terraform apply`时，Terraform会自动读取该文件中的变量值

除了使用`terraform.tfvars`文件外，还可以通过以下方式设置变量值：

1. 命令行参数：`terraform apply -var="vpc_name=my-vpc"`
2. 环境变量：`export TF_VAR_vpc_name=my-vpc`
3. 自定义命名的变量文件：`terraform apply -var-file="custom.tfvars"`

> 注意：如果同一个变量通过多种方式进行设置，Terraform会按照以下优先级使用变量值：命令行参数 > 变量文件 > 环境变量 > 默认值。

### x. 初始化并应用Terraform配置

完成以上脚本配置后，执行以下步骤来创建资源：

1. 运行 `terraform init` 初始化环境
2. 运行 `terraform plan` 查看资源创建计划
3. 确认资源计划无误后，运行 `terraform apply` 开始创建{本实践主体对象}
4. 运行 `terraform show` 查看已创建的{本实践主体对象}

## 参考信息

- [华为云{产品}产品文档](https://support.huaweicloud.com/{product}/index.html)
- [华为云Provider文档](https://registry.terraform.io/providers/huaweicloud/huaweicloud/latest/docs)
- [{服务名}{场景}最佳实践源码参考](https://github.com/{B_REPO}/tree/{B_DEFAULT_BRANCH}/examples/{service}/{practice})

（英文正文 H1 必须为 `Deploy {Scene}`；对应源码锚文本必须为：`Best Practice Source Code Reference For {ServiceName} {PracticeObject}`，例如 `Best Practice Source Code Reference For AAD Black/White Lists`——含服务名、**不含** `Deploy`。）
