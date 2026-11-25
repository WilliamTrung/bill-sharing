const I18n = (() => {
  const STORAGE_KEY = "bs.lang";
  const SUPPORTED = ["en", "vi"];
  const DEFAULT_LANG = "en";

  const en = {
    "app.name": "Bill Sharing",
    "app.eyebrow.overview": "Overview",
    "app.eyebrow.admin": "Admin",
    "app.eyebrow.newExpense": "New expense",
    "app.eyebrow.account": "Account",

    "nav.bills": "Bills",
    "nav.newBill": "New bill",
    "nav.admin": "Members & groups",
    "nav.profile": "Profile",
    "nav.signOut": "Sign out",
    "nav.switchLanguage": "Switch language",
    "nav.primary": "Primary",

    "action.cancel": "Cancel",
    "action.close": "Close",
    "action.edit": "Edit",
    "action.details": "Details",
    "action.addBill": "Add bill",
    "action.addMember": "Add member",
    "action.newGroup": "New group",

    "state.active": "Active",
    "state.inactive": "Inactive",

    "msg.somethingWrong": "Something went wrong",
    "msg.networkError": "Network error: could not reach the server",
    "msg.sessionExpired": "Your session expired. Sign in again.",
    "msg.requestFailed": "Request failed ({status})",
    "msg.uploadFailed": "Upload failed ({status})",

    "unit.bill": "bill",
    "unit.bills": "bills",
    "unit.member": "member",
    "unit.members": "members",
    "unit.attachment": "attachment",
    "unit.attachments": "attachments",
    "unit.participant": "participant",
    "unit.participants": "participants",

    "home.docTitle": "Bill Sharing",
    "home.heading": "Shared bills",
    "home.sub": "Ongoing shared bills, who shared them, and their status.",
    "home.signIn": "Sign in",

    "public.heading": "Shared bills",
    "public.sub": "Everything being shared right now, who is involved, and its status.",
    "public.ongoing": "Ongoing",
    "public.all": "All",
    "public.ongoingCount": "Ongoing",
    "public.totalTracked": "Total tracked",
    "public.membersCount": "Shared with",
    "public.members": "Shared with",
    "public.none": "No bills shared yet.",
    "public.noOngoing": "No ongoing bills right now.",
    "public.signInToAdd": "Sign in to add a bill",
    "public.attachments": "{count} attachments",
    "public.statusOpen": "Ongoing",
    "public.statusSettled": "Settled",

    "shares.member": "Member",
    "shares.source": "Source",
    "shares.amount": "Share",
    "shares.show": "Show member shares",
    "shares.hide": "Hide member shares",
    "shares.none": "No member shares recorded for this bill.",

    "profile.docTitle": "Profile · Bill Sharing",
    "profile.heading": "Your profile",
    "profile.sub": "Update the name others see, how you sign in, and your password.",
    "profile.details": "Profile details",
    "profile.detailsHint": "Your display name, sign-in username and email.",
    "profile.displayName": "Display name",
    "profile.displayNameHint": "Shown on bills, balances and the members list.",
    "profile.username": "Username",
    "profile.usernameHint": "You can sign in with this or your email.",
    "profile.email": "Email",
    "profile.language": "Default language",
    "profile.languageHint": "Used when you open the app on a device that has no language picked yet.",
    "profile.langEn": "English",
    "profile.langVi": "Tiếng Việt",
    "profile.publicLabel": "Public profile",
    "profile.publicHint": "When off, only you and admins can see your email, phone and notes.",
    "profile.save": "Save changes",
    "profile.saving": "Saving…",
    "profile.saved": "Profile updated.",
    "profile.errRequired": "Fill in display name, username and email.",
    "profile.passwordTitle": "Change password",
    "profile.passwordHint": "Use at least 8 characters.",
    "profile.currentPassword": "Current password",
    "profile.newPassword": "New password",
    "profile.confirmPassword": "Confirm new password",
    "profile.changePassword": "Change password",
    "profile.changing": "Changing…",
    "profile.passwordChanged": "Password changed.",
    "profile.passwordUnavailable": "This account has no password yet, so it cannot change one.",
    "profile.errPasswordRequired": "Enter your current and new password.",
    "profile.errMismatch": "The new passwords do not match.",
    "profile.errTooShort": "The new password must be at least 8 characters.",

    "login.docTitle": "Sign in · Bill Sharing",
    "login.heading": "Sign in",
    "login.sub": "Sign in to add bills and manage members and groups.",
    "login.identifier": "Username or email",
    "login.identifierPlaceholder": "you@example.com",
    "login.password": "Password",
    "login.submit": "Sign in",
    "login.signingIn": "Signing in…",
    "login.errRequired": "Enter your username or email and password.",
    "login.errFailed": "Could not sign in.",
    "server.authRequired": "Authentication required",
    "server.invalidCredentials": "Incorrect username, email or password",
    "server.currentPasswordWrong": "Current password is incorrect",
    "server.usernameTaken": "Username is already taken",
    "server.emailTaken": "Email is already in use",
    "server.tooManyAttempts": "Too many attempts. Try again later.",

    "index.docTitle": "Dashboard · Bill Sharing",
    "index.heading": "Shared expenses",
    "index.sub": "Track who paid, who owes, and every receipt in one place.",
    "index.totalTracked": "Total tracked",
    "index.activeMembers": "Active members",
    "index.total": "total",
    "index.balancesStat": "Balances",
    "index.creditorsOwing": "{creditors} owed · {debtors} owing",
    "index.balancesSee": "See net balances below",
    "index.netBalances": "Net balances",
    "index.netBalancesHint": "Positive means this person is owed money overall.",
    "index.member": "Member",
    "index.paid": "Paid",
    "index.owed": "Owed",
    "index.net": "Net",
    "index.noMembers": "No members yet. Add members on the /admin page.",
    "index.billHistory": "Bill history",
    "index.billHistoryHint": "Newest expenses first.",
    "index.bill": "Bill",
    "index.date": "Date",
    "index.paidBy": "Paid by",
    "index.split": "Split",
    "index.status": "Status",
    "index.amount": "Amount",
    "index.noBills": "No bills recorded yet.",
    "index.noBillsLink": "Add the first one",
    "index.splitEven": "Even",
    "index.splitRatio": "Ratio",

    "admin.docTitle": "Members & groups · Bill Sharing",
    "admin.heading": "Members & groups",
    "admin.sub": "People who share expenses, and the composite groups they form.",
    "admin.members": "Members",
    "admin.membersHint": "Deactivating a member keeps their past bills intact.",
    "admin.showInactive": "Show inactive",
    "admin.name": "Name",
    "admin.contact": "Contact",
    "admin.groups": "Groups",
    "admin.actions": "Actions",
    "admin.noMembers": "No members yet. Add the first person above.",
    "admin.groupsTitle": "Groups",
    "admin.groupsHint": "A group acts as a single participant on bills.",
    "admin.noGroups": "No groups yet. Create one to split bills between couples or families.",
    "admin.member": "Member",
    "admin.createGroupWith": "Create group with this member",
    "admin.addMemberTitle": "Add member",
    "admin.editMemberTitle": "Edit member",
    "admin.phone": "Phone",
    "admin.email": "Email",
    "admin.notes": "Notes",
    "admin.status": "Status",
    "admin.saveMember": "Save member",
    "admin.memberAdded": "Member added.",
    "admin.memberUpdated": "Member updated.",
    "admin.groupCreated": "Group created.",
    "admin.groupUpdated": "Group updated.",
    "admin.errNameRequired": "Name is required.",
    "admin.groupName": "Group name",
    "admin.groupNamePlaceholder": "e.g. Alex & Sam",
    "admin.internalSplit": "Internal split",
    "admin.internalSplitHint": "How this group's share is divided among its own members.",
    "admin.membersLabel": "Members",
    "admin.pickTwo": "Pick at least two active members.",
    "admin.newGroupTitle": "New group",
    "admin.editGroupTitle": "Edit group",
    "admin.saveGroup": "Save group",
    "admin.errGroupNameRequired": "Group name is required.",
    "admin.errRatio": "Ratio for {name} must be greater than zero.",
    "admin.errPickTwo": "Pick at least two members.",
    "admin.addTwoActives": "Add at least two active members before creating a group.",
    "admin.notInGroup": "Not in any group yet",
    "admin.activateFirst": "Activate this member first",
    "admin.splitEven": "Even",
    "admin.splitRatio": "Ratio",
    "admin.internalEven": "internal even",
    "admin.internalRatio": "internal ratio",
    "admin.ratio": "Ratio",
    "admin.none": "None",
    "admin.ratioFor": "Ratio for {name}",
    "admin.activate": "Active",
    "admin.sendInvite": "Send invite",
    "admin.sendInviteHint": "Email an activation link so this member can set a password",
    "admin.resendInvite": "Resend invite",
    "admin.resendInviteHint": "Email a fresh activation link so this member can set a password",
    "admin.inviteSending": "Sending…",
    "admin.inviteSent": "Invite sent.",

    "activate.docTitle": "Activate account · Bill Sharing",
    "activate.heading": "Set your password",
    "activate.sub": "Choose a password to finish setting up your account.",
    "activate.greeting": "Hi {name}, choose a password to finish setting up your account.",
    "activate.password": "Password",
    "activate.passwordHint": "At least 8 characters.",
    "activate.confirm": "Confirm password",
    "activate.submit": "Activate account",
    "activate.activating": "Activating…",
    "activate.success": "Your account is ready.",
    "activate.continue": "Continue to the dashboard",
    "activate.errTooShort": "The password must be at least 8 characters.",
    "activate.errMismatch": "The passwords do not match.",
    "activate.missingToken": "This page needs an invitation link.",
    "activate.invalid": "This invitation link is invalid or expired.",
    "activate.used": "This invitation has already been used. Try signing in instead.",

    "bill.docTitle": "New bill · Bill Sharing",
    "bill.heading": "Add a bill",
    "bill.sub": "Pick who shares it, choose how to split, and attach the receipts.",
    "bill.details": "Bill details",
    "bill.title": "Title",
    "bill.titlePlaceholder": "Dinner at Nha Hang",
    "bill.description": "Description",
    "bill.descriptionPlaceholder": "What was this for?",
    "bill.totalAmount": "Total amount",
    "bill.date": "Date",
    "bill.paidBy": "Paid by",
    "bill.payerPlaceholder": "Select who paid…",
    "bill.payerPickerTitle": "Who paid?",
    "bill.payerPickerHint": "Search by name or email, then pick one person.",
    "bill.payerSearch": "Search",
    "bill.payerSearchPlaceholder": "Name or email",
    "bill.payerNoMatches": "No matching people.",
    "bill.payerSelected": "Selected",
    "bill.changePayer": "Change",
    "bill.youPill": "You",
    "bill.paidByHint": "The person who covered the bill upfront.",
    "bill.splitMode": "Split mode",
    "bill.even": "Even",
    "bill.ratio": "Ratio",
    "bill.saveBill": "Save bill",
    "bill.saving": "Saving…",
    "bill.saved": "Bill saved.",
    "bill.previewTitle": "Live split preview",
    "bill.previewHint": "Updates as you change amounts, weights and selections.",
    "bill.noParticipants": "No participants",
    "bill.incomplete": "Incomplete",
    "bill.total": "Total",
    "bill.participantsTitle": "Participants",
    "bill.participantsHint": "Select people and/or groups. Overlapping selections are disabled.",
    "bill.searchParticipants": "Search participants",
    "bill.searchPlaceholder": "Search by name",
    "bill.noMatches": "No matching participants.",
    "bill.noActiveMembers": "No active members yet.",
    "bill.addMembersFirst": "Add members first",
    "bill.evidence": "Receipts & evidence",
    "bill.evidenceHint": "JPEG, PNG, WebP or PDF. Up to 10 files.",
    "bill.dropzone": "Drop files here or click to browse",
    "bill.maxSize": "Max 25 MB per bill",
    "bill.removeFile": "Remove {name}",
    "bill.errTitleRequired": "Bill title is required.",
    "bill.errAmount": "Enter a total amount greater than zero.",
    "bill.errDate": "Pick the bill date.",
    "bill.errPayer": "Select who paid.",
    "bill.errParticipants": "Select at least one participant.",
    "bill.errSelectOne": "Select at least one participant",
    "bill.errTotal": "Enter a total amount greater than zero",
    "bill.errWeightZero": "Every weight must be greater than zero",
    "bill.errSplitFail": "Could not split this amount",
    "bill.disabledPrefix": "Disabled: {reason}",
    "bill.includedIn": "{name} is already included in “{group}”",
    "bill.sharesWith": "Shares {names} with “{group}”",
    "bill.selectedIndividually": "{name} is already selected individually",
    "bill.weighted": "weighted",
    "bill.weight": "Weight",
    "bill.groupPill": "Group",
    "bill.groupSummary": "{count} members · {kind}",
    "bill.maxFiles": "Only {count} files allowed.",
    "bill.unsupportedType": "{name}: unsupported type (JPEG, PNG, WebP or PDF).",
    "bill.overLimit": "Attachments exceed the 25 MB limit.",
    "bill.allocMismatch": "Allocated total does not match the bill total.",
    "bill.weightTitle": "weight {weight}",

    "detail.docTitle": "Bill · Bill Sharing",
    "detail.heading": "Bill details",
    "detail.sub": "Itemized breakdown and uploaded evidence.",
    "detail.backToBills": "Back to bills",
    "detail.deleteBill": "Delete bill",
    "detail.deleted": "Bill deleted.",
    "detail.confirmDelete": "Delete “{title}”? Uploaded evidence will be removed too.",
    "detail.notFound": "Bill not found.",
    "detail.total": "Total amount",
    "detail.paidBy": "Paid by",
    "detail.date": "Date",
    "detail.split": "Split",
    "detail.status": "Status",
    "detail.markSettled": "Mark settled",
    "detail.markOpen": "Mark ongoing",
    "detail.statusUpdated": "Bill status updated.",
    "detail.created": "Recorded",
    "detail.participants": "Top-level participants",
    "detail.memberShares": "Per-member breakdown",
    "detail.share": "Share",
    "detail.fromGroup": "from {group}",
    "detail.direct": "Direct",
    "detail.evidence": "Evidence",
    "detail.noEvidence": "No receipts attached to this bill.",
    "detail.viewFile": "Open {name}",
    "detail.sumShares": "Sum of shares",
    "detail.ratioMode": "ratio",
    "detail.evenMode": "even",
    "detail.missing": "No participants recorded for this bill.",
  };

  const vi = {
    "app.name": "Chia Hoá Đơn",
    "app.eyebrow.overview": "Tổng quan",
    "app.eyebrow.admin": "Quản trị",
    "app.eyebrow.newExpense": "Chi phí mới",
    "app.eyebrow.account": "Tài khoản",

    "nav.bills": "Hoá đơn",
    "nav.newBill": "Tạo hoá đơn",
    "nav.admin": "Thành viên & nhóm",
    "nav.profile": "Hồ sơ",
    "nav.signOut": "Đăng xuất",
    "nav.switchLanguage": "Đổi ngôn ngữ",
    "nav.primary": "Chính",

    "action.cancel": "Huỷ",
    "action.close": "Đóng",
    "action.edit": "Sửa",
    "action.details": "Chi tiết",
    "action.addBill": "Thêm hoá đơn",
    "action.addMember": "Thêm thành viên",
    "action.newGroup": "Nhóm mới",

    "state.active": "Đang hoạt động",
    "state.inactive": "Ngừng hoạt động",

    "msg.somethingWrong": "Đã có lỗi xảy ra",
    "msg.networkError": "Lỗi mạng: không kết nối được tới máy chủ",
    "msg.sessionExpired": "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
    "msg.requestFailed": "Yêu cầu thất bại ({status})",
    "msg.uploadFailed": "Tải lên thất bại ({status})",

    "unit.bill": "hoá đơn",
    "unit.bills": "hoá đơn",
    "unit.member": "thành viên",
    "unit.members": "thành viên",
    "unit.attachment": "tệp đính kèm",
    "unit.attachments": "tệp đính kèm",
    "unit.participant": "người tham gia",
    "unit.participants": "người tham gia",

    "home.docTitle": "Chia Hoá Đơn",
    "home.heading": "Hoá đơn chung",
    "home.sub": "Các hoá đơn chung đang mở, ai cùng chia sẻ và trạng thái của chúng.",
    "home.signIn": "Đăng nhập",

    "public.heading": "Hoá đơn chung",
    "public.sub": "Mọi hoá đơn đang được chia sẻ, ai cùng tham gia và trạng thái của chúng.",
    "public.ongoing": "Đang mở",
    "public.all": "Tất cả",
    "public.ongoingCount": "Đang mở",
    "public.totalTracked": "Tổng đã ghi nhận",
    "public.membersCount": "Cùng chia sẻ",
    "public.members": "Cùng chia sẻ",
    "public.none": "Chưa có hoá đơn chung nào.",
    "public.noOngoing": "Hiện không có hoá đơn nào đang mở.",
    "public.signInToAdd": "Đăng nhập để thêm hoá đơn",
    "public.attachments": "{count} tệp đính kèm",
    "public.statusOpen": "Đang mở",
    "public.statusSettled": "Đã tất toán",

    "shares.member": "Thành viên",
    "shares.source": "Nguồn",
    "shares.amount": "Phần phải trả",
    "shares.show": "Xem phần của từng thành viên",
    "shares.hide": "Ẩn phần của từng thành viên",
    "shares.none": "Hoá đơn này chưa có phần chia nào.",

    "profile.docTitle": "Hồ sơ · Chia Hoá Đơn",
    "profile.heading": "Hồ sơ của bạn",
    "profile.sub": "Cập nhật tên hiển thị, cách đăng nhập và mật khẩu của bạn.",
    "profile.details": "Thông tin hồ sơ",
    "profile.detailsHint": "Tên hiển thị, tên đăng nhập và email của bạn.",
    "profile.displayName": "Tên hiển thị",
    "profile.displayNameHint": "Hiển thị trên hoá đơn, số dư và danh sách thành viên.",
    "profile.username": "Tên đăng nhập",
    "profile.usernameHint": "Bạn có thể đăng nhập bằng tên này hoặc email.",
    "profile.email": "Email",
    "profile.language": "Ngôn ngữ mặc định",
    "profile.languageHint": "Dùng khi bạn mở ứng dụng trên thiết bị chưa chọn ngôn ngữ.",
    "profile.langEn": "English",
    "profile.langVi": "Tiếng Việt",
    "profile.publicLabel": "Hồ sơ công khai",
    "profile.publicHint": "Khi tắt, chỉ bạn và quản trị viên thấy email, số điện thoại và ghi chú của bạn.",
    "profile.save": "Lưu thay đổi",
    "profile.saving": "Đang lưu…",
    "profile.saved": "Đã cập nhật hồ sơ.",
    "profile.errRequired": "Vui lòng nhập tên hiển thị, tên đăng nhập và email.",
    "profile.passwordTitle": "Đổi mật khẩu",
    "profile.passwordHint": "Dùng ít nhất 8 ký tự.",
    "profile.currentPassword": "Mật khẩu hiện tại",
    "profile.newPassword": "Mật khẩu mới",
    "profile.confirmPassword": "Nhập lại mật khẩu mới",
    "profile.changePassword": "Đổi mật khẩu",
    "profile.changing": "Đang đổi…",
    "profile.passwordChanged": "Đã đổi mật khẩu.",
    "profile.passwordUnavailable": "Tài khoản này chưa có mật khẩu nên không thể đổi.",
    "profile.errPasswordRequired": "Vui lòng nhập mật khẩu hiện tại và mật khẩu mới.",
    "profile.errMismatch": "Mật khẩu mới không khớp.",
    "profile.errTooShort": "Mật khẩu mới phải có ít nhất 8 ký tự.",

    "login.docTitle": "Đăng nhập · Chia Hoá Đơn",
    "login.heading": "Đăng nhập",
    "login.sub": "Đăng nhập để thêm hoá đơn và quản lý thành viên, nhóm.",
    "login.identifier": "Tên đăng nhập hoặc email",
    "login.identifierPlaceholder": "ban@example.com",
    "login.password": "Mật khẩu",
    "login.submit": "Đăng nhập",
    "login.signingIn": "Đang đăng nhập…",
    "login.errRequired": "Vui lòng nhập tên đăng nhập hoặc email và mật khẩu.",
    "login.errFailed": "Không thể đăng nhập.",
    "server.authRequired": "Yêu cầu đăng nhập",
    "server.invalidCredentials": "Sai tên đăng nhập, email hoặc mật khẩu",
    "server.currentPasswordWrong": "Mật khẩu hiện tại không đúng",
    "server.usernameTaken": "Tên đăng nhập đã được dùng",
    "server.emailTaken": "Email đã được dùng",
    "server.tooManyAttempts": "Quá nhiều lần thử. Vui lòng thử lại sau.",

    "index.docTitle": "Tổng quan · Chia Hoá Đơn",
    "index.heading": "Chi phí chung",
    "index.sub": "Theo dõi ai đã trả, ai còn nợ và mọi hoá đơn ở một nơi.",
    "index.totalTracked": "Tổng đã ghi nhận",
    "index.activeMembers": "Thành viên hoạt động",
    "index.total": "tổng số",
    "index.balancesStat": "Số dư",
    "index.creditorsOwing": "{creditors} được trả · {debtors} còn nợ",
    "index.balancesSee": "Xem số dư ròng bên dưới",
    "index.netBalances": "Số dư ròng",
    "index.netBalancesHint": "Số dương nghĩa là người này nhìn chung được trả lại tiền.",
    "index.member": "Thành viên",
    "index.paid": "Đã trả",
    "index.owed": "Còn nợ",
    "index.net": "Ròng",
    "index.noMembers": "Chưa có thành viên. Hãy thêm thành viên tại trang /admin.",
    "index.billHistory": "Lịch sử hoá đơn",
    "index.billHistoryHint": "Chi phí mới nhất trước.",
    "index.bill": "Hoá đơn",
    "index.date": "Ngày",
    "index.paidBy": "Người trả",
    "index.split": "Cách chia",
    "index.status": "Trạng thái",
    "index.amount": "Số tiền",
    "index.noBills": "Chưa có hoá đơn nào.",
    "index.noBillsLink": "Thêm hoá đơn đầu tiên",
    "index.splitEven": "Chia đều",
    "index.splitRatio": "Theo tỷ lệ",

    "admin.docTitle": "Thành viên & nhóm · Chia Hoá Đơn",
    "admin.heading": "Thành viên & nhóm",
    "admin.sub": "Những người cùng chia sẻ chi phí và các nhóm ghép mà họ tạo thành.",
    "admin.members": "Thành viên",
    "admin.membersHint": "Ngừng hoạt động một thành viên sẽ giữ nguyên các hoá đơn cũ.",
    "admin.showInactive": "Hiện cả người ngừng hoạt động",
    "admin.name": "Tên",
    "admin.contact": "Liên hệ",
    "admin.groups": "Nhóm",
    "admin.actions": "Thao tác",
    "admin.noMembers": "Chưa có thành viên. Hãy thêm người đầu tiên ở trên.",
    "admin.groupsTitle": "Nhóm",
    "admin.groupsHint": "Một nhóm được tính là một người tham gia trên hoá đơn.",
    "admin.noGroups": "Chưa có nhóm. Tạo một nhóm để chia hoá đơn giữa các cặp đôi hoặc gia đình.",
    "admin.member": "Thành viên",
    "admin.createGroupWith": "Tạo nhóm với thành viên này",
    "admin.addMemberTitle": "Thêm thành viên",
    "admin.editMemberTitle": "Sửa thành viên",
    "admin.phone": "Số điện thoại",
    "admin.email": "Email",
    "admin.notes": "Ghi chú",
    "admin.status": "Trạng thái",
    "admin.saveMember": "Lưu thành viên",
    "admin.memberAdded": "Đã thêm thành viên.",
    "admin.memberUpdated": "Đã cập nhật thành viên.",
    "admin.groupCreated": "Đã tạo nhóm.",
    "admin.groupUpdated": "Đã cập nhật nhóm.",
    "admin.errNameRequired": "Tên là bắt buộc.",
    "admin.groupName": "Tên nhóm",
    "admin.groupNamePlaceholder": "vd. Alex & Sam",
    "admin.internalSplit": "Cách chia nội bộ",
    "admin.internalSplitHint": "Phần của nhóm này được chia thế nào giữa các thành viên trong nhóm.",
    "admin.membersLabel": "Thành viên",
    "admin.pickTwo": "Chọn ít nhất hai thành viên đang hoạt động.",
    "admin.newGroupTitle": "Nhóm mới",
    "admin.editGroupTitle": "Sửa nhóm",
    "admin.saveGroup": "Lưu nhóm",
    "admin.errGroupNameRequired": "Tên nhóm là bắt buộc.",
    "admin.errRatio": "Tỷ lệ của {name} phải lớn hơn 0.",
    "admin.errPickTwo": "Vui lòng chọn ít nhất hai thành viên.",
    "admin.addTwoActives": "Cần ít nhất hai thành viên đang hoạt động trước khi tạo nhóm.",
    "admin.notInGroup": "Chưa thuộc nhóm nào",
    "admin.activateFirst": "Hãy kích hoạt thành viên này trước",
    "admin.splitEven": "Chia đều",
    "admin.splitRatio": "Theo tỷ lệ",
    "admin.internalEven": "chia đều nội bộ",
    "admin.internalRatio": "tỷ lệ nội bộ",
    "admin.ratio": "Tỷ lệ",
    "admin.none": "Không có",
    "admin.ratioFor": "Tỷ lệ cho {name}",
    "admin.activate": "Hoạt động",
    "admin.sendInvite": "Gửi lời mời",
    "admin.sendInviteHint": "Gửi email chứa liên kết kích hoạt để thành viên này đặt mật khẩu",
    "admin.resendInvite": "Gửi lại lời mời",
    "admin.resendInviteHint": "Gửi lại liên kết kích hoạt mới để thành viên này đặt mật khẩu",
    "admin.inviteSending": "Đang gửi…",
    "admin.inviteSent": "Đã gửi lời mời.",

    "activate.docTitle": "Kích hoạt tài khoản · Chia Hoá Đơn",
    "activate.heading": "Đặt mật khẩu",
    "activate.sub": "Chọn mật khẩu để hoàn tất việc thiết lập tài khoản.",
    "activate.greeting": "Chào {name}, hãy chọn mật khẩu để hoàn tất thiết lập tài khoản.",
    "activate.password": "Mật khẩu",
    "activate.passwordHint": "Ít nhất 8 ký tự.",
    "activate.confirm": "Nhập lại mật khẩu",
    "activate.submit": "Kích hoạt tài khoản",
    "activate.activating": "Đang kích hoạt…",
    "activate.success": "Tài khoản của bạn đã sẵn sàng.",
    "activate.continue": "Tiếp tục tới bảng điều khiển",
    "activate.errTooShort": "Mật khẩu phải có ít nhất 8 ký tự.",
    "activate.errMismatch": "Mật khẩu không khớp.",
    "activate.missingToken": "Trang này cần liên kết lời mời.",
    "activate.invalid": "Liên kết lời mời không hợp lệ hoặc đã hết hạn.",
    "activate.used": "Lời mời này đã được dùng. Hãy đăng nhập.",

    "bill.docTitle": "Tạo hoá đơn · Chia Hoá Đơn",
    "bill.heading": "Thêm hoá đơn",
    "bill.sub": "Chọn người cùng chia sẻ, cách chia và đính kèm hoá đơn gốc.",
    "bill.details": "Thông tin hoá đơn",
    "bill.title": "Tiêu đề",
    "bill.titlePlaceholder": "Ăn tối ở Nhà Hàng",
    "bill.description": "Mô tả",
    "bill.descriptionPlaceholder": "Hoá đơn này cho việc gì?",
    "bill.totalAmount": "Tổng số tiền",
    "bill.date": "Ngày",
    "bill.paidBy": "Người trả",
    "bill.payerPlaceholder": "Chọn người đã trả…",
    "bill.payerPickerTitle": "Ai đã trả?",
    "bill.payerPickerHint": "Tìm theo tên hoặc email, sau đó chọn một người.",
    "bill.payerSearch": "Tìm kiếm",
    "bill.payerSearchPlaceholder": "Tên hoặc email",
    "bill.payerNoMatches": "Không có người phù hợp.",
    "bill.payerSelected": "Đã chọn",
    "bill.changePayer": "Đổi",
    "bill.youPill": "Bạn",
    "bill.paidByHint": "Người đã trả trước cho hoá đơn này.",
    "bill.splitMode": "Cách chia",
    "bill.even": "Chia đều",
    "bill.ratio": "Theo tỷ lệ",
    "bill.saveBill": "Lưu hoá đơn",
    "bill.saving": "Đang lưu…",
    "bill.saved": "Đã lưu hoá đơn.",
    "bill.previewTitle": "Xem trước cách chia",
    "bill.previewHint": "Cập nhật khi bạn đổi số tiền, tỷ lệ và lựa chọn.",
    "bill.noParticipants": "Chưa có người tham gia",
    "bill.incomplete": "Chưa đủ thông tin",
    "bill.total": "Tổng",
    "bill.participantsTitle": "Người tham gia",
    "bill.participantsHint": "Chọn từng người và/hoặc nhóm. Các lựa chọn trùng lặp sẽ bị vô hiệu hoá.",
    "bill.searchParticipants": "Tìm người tham gia",
    "bill.searchPlaceholder": "Tìm theo tên",
    "bill.noMatches": "Không có người tham gia phù hợp.",
    "bill.noActiveMembers": "Chưa có thành viên đang hoạt động.",
    "bill.addMembersFirst": "Thêm thành viên trước",
    "bill.evidence": "Hoá đơn gốc & bằng chứng",
    "bill.evidenceHint": "JPEG, PNG, WebP hoặc PDF. Tối đa 10 tệp.",
    "bill.dropzone": "Kéo tệp vào đây hoặc bấm để chọn",
    "bill.maxSize": "Tối đa 25 MB mỗi hoá đơn",
    "bill.removeFile": "Xoá {name}",
    "bill.errTitleRequired": "Tiêu đề hoá đơn là bắt buộc.",
    "bill.errAmount": "Nhập tổng số tiền lớn hơn 0.",
    "bill.errDate": "Chọn ngày của hoá đơn.",
    "bill.errPayer": "Chọn người đã trả.",
    "bill.errParticipants": "Chọn ít nhất một người tham gia.",
    "bill.errSelectOne": "Chọn ít nhất một người tham gia",
    "bill.errTotal": "Nhập tổng số tiền lớn hơn 0",
    "bill.errWeightZero": "Mọi tỷ lệ phải lớn hơn 0",
    "bill.errSplitFail": "Không thể chia số tiền này",
    "bill.disabledPrefix": "Đã vô hiệu hoá: {reason}",
    "bill.includedIn": "{name} đã có trong “{group}”",
    "bill.sharesWith": "Trùng {names} với “{group}”",
    "bill.selectedIndividually": "{name} đã được chọn riêng",
    "bill.weighted": "có tỷ lệ",
    "bill.weight": "Tỷ lệ",
    "bill.groupPill": "Nhóm",
    "bill.groupSummary": "{count} thành viên · {kind}",
    "bill.maxFiles": "Chỉ cho phép {count} tệp.",
    "bill.unsupportedType": "{name}: định dạng không hỗ trợ (JPEG, PNG, WebP hoặc PDF).",
    "bill.overLimit": "Tệp đính kèm vượt quá giới hạn 25 MB.",
    "bill.allocMismatch": "Tổng đã chia không khớp với tổng hoá đơn.",
    "bill.weightTitle": "tỷ lệ {weight}",

    "detail.docTitle": "Hoá đơn · Chia Hoá Đơn",
    "detail.heading": "Chi tiết hoá đơn",
    "detail.sub": "Bảng chia chi tiết và bằng chứng đã tải lên.",
    "detail.backToBills": "Quay lại danh sách",
    "detail.deleteBill": "Xoá hoá đơn",
    "detail.deleted": "Đã xoá hoá đơn.",
    "detail.confirmDelete": "Xoá “{title}”? Các tệp bằng chứng cũng sẽ bị xoá.",
    "detail.notFound": "Không tìm thấy hoá đơn.",
    "detail.total": "Tổng số tiền",
    "detail.paidBy": "Người trả",
    "detail.date": "Ngày",
    "detail.split": "Cách chia",
    "detail.status": "Trạng thái",
    "detail.markSettled": "Đánh dấu đã tất toán",
    "detail.markOpen": "Đánh dấu đang mở",
    "detail.statusUpdated": "Đã cập nhật trạng thái hoá đơn.",
    "detail.created": "Ghi nhận lúc",
    "detail.participants": "Người tham gia cấp cao",
    "detail.memberShares": "Chi tiết từng thành viên",
    "detail.share": "Phần phải trả",
    "detail.fromGroup": "từ {group}",
    "detail.direct": "Trực tiếp",
    "detail.evidence": "Bằng chứng",
    "detail.noEvidence": "Hoá đơn này chưa có tệp đính kèm.",
    "detail.viewFile": "Mở {name}",
    "detail.sumShares": "Tổng các phần",
    "detail.ratioMode": "tỷ lệ",
    "detail.evenMode": "chia đều",
    "detail.missing": "Hoá đơn này chưa có người tham gia.",
  };

  const dict = { en, vi };
  const serverMessages = {
    "Authentication required": "server.authRequired",
    "Incorrect username, email or password": "server.invalidCredentials",
    "Current password is incorrect": "server.currentPasswordWrong",
    "Username is already taken": "server.usernameTaken",
    "Email is already in use": "server.emailTaken",
    "Too many attempts. Try again later.": "server.tooManyAttempts",
  };

  let lang = DEFAULT_LANG;
  let profileLang = null;
  const listeners = new Set();

  function normalize(code) {
    const value = String(code || "").toLowerCase().split(/[-_]/)[0];
    return SUPPORTED.includes(value) ? value : null;
  }

  function detect() {
    let stored = null;
    try {
      stored = localStorage.getItem(STORAGE_KEY);
    } catch (err) {
      stored = null;
    }
    const fromStore = normalize(stored);
    if (fromStore) return fromStore;

    const fromQuery = normalize(new URLSearchParams(window.location.search).get("lang"));
    if (fromQuery) return fromQuery;

    const fromProfile = normalize(profileLang);
    if (fromProfile) return fromProfile;

    for (const candidate of navigator.languages || [navigator.language || ""]) {
      const match = normalize(candidate);
      if (match) return match;
    }
    return DEFAULT_LANG;
  }

  function t(key, vars) {
    const table = dict[lang] || dict[DEFAULT_LANG];
    let value = table[key];
    if (value === undefined) value = dict[DEFAULT_LANG][key];
    if (value === undefined) return key;
    if (vars) {
      value = value.replace(/\{(\w+)\}/g, (match, name) =>
        vars[name] === undefined || vars[name] === null ? match : String(vars[name])
      );
    }
    return value;
  }

  function translateServer(message) {
    const key = serverMessages[String(message || "").trim()];
    return key ? t(key) : message;
  }

  function apply(root) {
    const scope = root || document;

    scope.querySelectorAll("[data-i18n]").forEach((node) => {
      node.textContent = t(node.dataset.i18n);
    });
    scope.querySelectorAll("[data-i18n-placeholder]").forEach((node) => {
      node.setAttribute("placeholder", t(node.dataset.i18nPlaceholder));
    });
    scope.querySelectorAll("[data-i18n-title]").forEach((node) => {
      node.setAttribute("title", t(node.dataset.i18nTitle));
    });
    scope.querySelectorAll("[data-i18n-aria-label]").forEach((node) => {
      node.setAttribute("aria-label", t(node.dataset.i18nAriaLabel));
    });

    document.documentElement.lang = lang === "vi" ? "vi" : "en";
    const title = document.querySelector("title[data-i18n]");
    if (title) document.title = t(title.dataset.i18n);
  }

  function setLang(code) {
    const next = normalize(code);
    if (!next || next === lang) return;
    lang = next;
    try {
      localStorage.setItem(STORAGE_KEY, lang);
    } catch (err) {
      /* storage may be unavailable; language still applies for this page */
    }
    apply();
    listeners.forEach((listener) => listener(lang));
  }

  function onChange(listener) {
    listeners.add(listener);
  }

  function mountSwitcher() {
    document.querySelectorAll("[data-lang-switcher]").forEach((container) => {
      if (container.dataset.ready === "true") {
        return;
      }
      container.dataset.ready = "true";

      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "btn btn-quiet lang-toggle";
      btn.setAttribute("data-i18n-aria-label", "nav.switchLanguage");

      const render = () => {
        btn.textContent = lang === "vi" ? "EN" : "VI";
        btn.setAttribute("aria-label", t("nav.switchLanguage"));
        btn.title = t("nav.switchLanguage");
      };

      btn.addEventListener("click", () => setLang(lang === "vi" ? "en" : "vi"));
      container.appendChild(btn);
      render();
      listeners.add(render);
    });
  }

  function useDefault(code) {
    const next = normalize(code);
    if (next) profileLang = next;
  }

  function init() {
    lang = detect();
    apply();
    mountSwitcher();
  }

  return {
    init,
    t,
    apply,
    setLang,
    useDefault,
    onChange,
    translateServer,
    lang: () => lang,
    getLocale: () => (lang === "vi" ? "vi-VN" : "en-US"),
  };
})();
