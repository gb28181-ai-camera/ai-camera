/// 登录请求体（对应后端 `request.LoginReq`）。
///
/// 注意：后端 JSON 字段为 `user_name`，**不是** `username`。
class LoginReq {
  /// 构造登录请求。
  LoginReq({required this.userName, required this.password});

  /// 登录名。
  final String userName;

  /// 密码（明文，由 HTTPS 保护）。
  final String password;

  /// 序列化为后端可解析的 JSON。
  Map<String, dynamic> toJson() => <String, dynamic>{
        'user_name': userName,
        'password': password,
      };

  /// 从 JSON 反序列化（测试与回显用）。
  factory LoginReq.fromJson(Map<String, dynamic> json) => LoginReq(
        userName: (json['user_name'] ?? '').toString(),
        password: (json['password'] ?? '').toString(),
      );

  @override
  String toString() => 'LoginReq(user_name: $userName)';
}

/// 更新个人资料请求体（对应后端 `request.UpdateProfileReq`）。
class UpdateProfileReq {
  /// 构造资料请求。
  const UpdateProfileReq({
    required this.name,
    this.email = '',
    this.phone = '',
    this.avatar = '',
  });

  /// 姓名。
  final String name;
  /// 邮箱。
  final String email;
  /// 手机号。
  final String phone;
  /// 头像地址。
  final String avatar;

  /// 序列化。
  Map<String, dynamic> toJson() => <String, dynamic>{
        'name': name,
        'email': email,
        'phone': phone,
        'avatar': avatar,
      };
}

/// 修改密码请求体（对应后端 `request.ChangePasswordReq`）。
class ChangePasswordReq {
  /// 构造修改密码请求。
  const ChangePasswordReq({
    required this.oldPassword,
    required this.newPassword,
  });

  /// 原密码。
  final String oldPassword;
  /// 新密码。
  final String newPassword;

  /// 序列化。
  Map<String, dynamic> toJson() => <String, dynamic>{
        'old_password': oldPassword,
        'new_password': newPassword,
      };
}

/// 风险/告警处理请求体（对应 `request.HandleRiskReq` / `request.HandleAlarmReq`）。
class HandleReq {
  /// 构造处理请求。
  const HandleReq({required this.handleResult, this.handleImage = ''});

  /// 处理结果（必填）。
  final String handleResult;
  /// 处理图片地址。
  final String handleImage;

  /// 序列化。
  Map<String, dynamic> toJson() => <String, dynamic>{
        'handle_result': handleResult,
        'handle_image': handleImage,
      };
}

/// 检测准确性确认请求体（对应 `request.UpdateAccurateReq`）。
class UpdateAccurateReq {
  /// 构造确认请求。
  const UpdateAccurateReq({required this.accurate});

  /// 确认风险。
  const UpdateAccurateReq.yes() : accurate = 'yes';

  /// 标记误报。
  const UpdateAccurateReq.no() : accurate = 'no';

  /// 取值 yes / no。
  final String accurate;

  /// 序列化。
  Map<String, dynamic> toJson() => <String, dynamic>{'accurate': accurate};
}

/// 完成巡检请求体（对应 `request.FinishInspectTaskReq`）。
class FinishInspectTaskReq {
  /// 构造完成巡检请求。
  const FinishInspectTaskReq({required this.result, this.resultImage = ''});

  /// 巡检结果（必填）。
  final String result;
  /// 结果图地址。
  final String resultImage;

  /// 序列化。
  Map<String, dynamic> toJson() => <String, dynamic>{
        'result': result,
        'result_image': resultImage,
      };
}
