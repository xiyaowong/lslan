# 审查提案工作流(待批准,不自动执行)

## 规则

1. 审查团队按领域产出改进项,每项一个 issue,标签 `proposal`(+ `ui`/`ux`/`engine`/`cleanup` 等)。
2. 提案发布后**只监听,不执行**。代码不得先改。
3. 批准信号二选一:
   - issue 上有一个 👍 reaction(任何人为该 issue 添加即可)
   - issue 评论区出现明确的批准回复(例如"批准"/"可以做"/"approve")
4. 未批准的提案保持 open;拿到批准后才开分支实现。
5. 被否决的(评论区说明不做)关闭并标 `wontfix`。

## 监听方式

用 `gh api repos/xiyaowong/lslan/issues/<n>/reactions --jq '.[].content'` 读 reaction,
用 `gh api repos/xiyaowong/lslan/issues/<n>/comments --jq '.[].body'` 读回复。

## 提案正文格式

- 现状与证据(file:line)
- 问题(为什么对用户/维护是负担)
- 方案(具体到数值或函数)
- 验收
- 代价与风险
