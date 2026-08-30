# -*- coding: utf-8 -*-
"""바로위키 시험용 데이터 심기

목록 화면이 비어 있으면 버그가 안 보이므로, 확인용 데이터를 한 번에 넣는다.

사용법
    1) 서버를 한 번 띄웠다 내려서 표를 만든다.  (.\\start.ps1 실행 후 Ctrl+C)
    2) python tests/baro_seed.py data.db
    3) 다시 서버를 띄우고 /baro 를 본다.

주의 : 시험용입니다. 실제로 쓰는 data.db 에는 넣지 마세요.
"""
import sys
import random
import sqlite3

path = sys.argv[1] if len(sys.argv) > 1 else "data.db"
con = sqlite3.connect(path)

NOW = "2026-08-27 12:00:00"
TODAY = "2026-08-27"
USERS = ["김기록", "이출처", "박검토", "최독립", "정철도"]
ROLES = ["sanction", "appeal", "reviewer", "privacy", "sysadmin"]
DOCS = ["최동환", "독립신문", "경부선", "한성순보", "3·1 운동", "김성범"]


def ins(table, **kw):
    cols = ["test"] + list(kw.keys())
    vals = [""] + list(kw.values())
    con.execute(
        "insert into %s (%s) values (%s)" % (table, ",".join(cols), ",".join(["?"] * len(vals))),
        vals,
    )


def main():
    for i, user in enumerate(USERS):
        ins("user_set", name="pw", id=user, data="x")
        ins("user_set", name="date", id=user, data="2026-0%d-01 09:00:00" % (i + 1))
        ins("baro_founder", user=user, number=str(i + 1), date=NOW, status="confirmed",
            first_event=str(i + 1), field="history", apply_date=NOW, note="")
        ins("baro_interest", user=user, field=random.choice(["history", "independence", "rail"]),
            kind="", date=NOW)
        ins("baro_impact", user=user, name="view", data=str(1000 * (i + 1)), date=NOW)
        ins("baro_user_badge", id=str(i + 1), user=user, badge="founder", date=NOW, detail="")
        ins("baro_calendar", user=user, date=TODAY, num=str(i + 2), detail='{"new_document":2}')
        ins("baro_role", user=user, role=ROLES[i], granted_by="system", date=NOW, memo="")

    for i, doc in enumerate(DOCS):
        ins("data", title=doc, data="== 개요 ==\n%s 문서입니다.[* 출처 %d]\n[[분류:인물]]" % (doc, i))
        ins("history", id="1", title=doc, data="본문", date=NOW, ip=USERS[i % 5],
            send="문서 작성", leng="+120", hide="", type="")
        ins("baro_contrib_event", id=str(i + 1), user=USERS[i % 5], kind="new_document", title=doc,
            rev="1", project="1", weight="100", detail="작성", date=NOW, status="pending", field="history")
        ins("baro_contrib_score", id=str(i + 1), user=USERS[i % 5], event=str(i + 1), base="100",
            source="20", novelty="30", review="0", survive="0", penalty="0", final="150",
            confirmed="on", confirm_date=NOW, date=NOW)
        ins("baro_citation", id=str(i + 1), title=doc, rev="1", kind="web",
            url="https://example.go.kr/%d" % i, author="", publisher="example.go.kr", pub_date="",
            access_date=TODAY, archive_url="", quality="primary", status="ok", date=NOW)
        ins("baro_doc_status", title=doc, status=random.choice(["need_source", "reviewed", "need_expand"]),
            reason="자동 판정", actor="system", date=NOW)
        ins("baro_doc_alias", id=str(i + 1), title=doc, alias="별칭%d" % i, kind="hanja", note="", date=NOW)
        ins("baro_doc_meta", title=doc, name="field", data="history")
        ins("baro_review", id=str(i + 1), title=doc, rev="1", user=USERS[i % 5], kind="edit",
            status="pending", reviewer="", review_date="", comment="가입 30일 이내", risk="45", date=NOW)
        ins("baro_watch", id=str(i + 1), user=USERS[0], kind="document", target=doc, date=NOW, mode="on")
        ins("baro_thanks", id=str(i + 1), from_user=USERS[(i + 1) % 5], to_user=USERS[i % 5],
            title=doc, rev="1", reason="좋은 문서", is_public="on", date=NOW)

    ins("baro_project", id="1", name="독립유공자 기록", field="independence", goal="공훈록 인물 기록",
        scope="18000명", dataset="공훈록", start_date=TODAY, end_date="", owner=USERS[0],
        status="active", target_count="18000", done_count="4310", rule="출처 필수",
        finish_cond="전원 기록", date=NOW, summary="", report="")

    for i in range(4):
        ins("baro_project_member", id=str(i + 1), project="1", user=USERS[i],
            role="member" if i else "owner", date=NOW, status="active")
        ins("baro_project_target", id=str(i + 1), project="1", name="대상 인물 %d" % i, note="",
            status="todo", doc="", date=NOW, assignee="")
        ins("baro_project_task", id=str(i + 1), project="1", kind="document", title="문서 요청 %d" % i,
            detail="만들어 주세요", difficulty="easy", est_time="30분", assignee="", status="open",
            doc="", date=NOW, done_date="", requester=USERS[0], field="independence")
        ins("baro_report", id=str(i + 1), kind="content", target_kind="document", target=DOCS[i],
            reporter=USERS[i], detail="오류가 있습니다", evidence="", status="new", case_id="",
            date=NOW, priority="normal", note="")
        ins("baro_case", id=str(i + 1), number="BW-2026-000%d" % (i + 1), kind="content",
            target_kind="document", target=DOCS[i], rule="principle", actor=USERS[0], status="open",
            action="", public_summary="검토 중", private_note="", date=NOW, close_date="",
            deadline="2026-09-10", appeal_status="", priority="normal")
        ins("baro_public_log", id=str(i + 1), kind="quarantine", actor=USERS[0], target=DOCS[i],
            reason="검토", detail="", date=NOW)
        ins("baro_audit_log", id=str(i + 1), actor=USERS[0], action="quarantine", target=DOCS[i],
            before_data="", after_data="", auth="admin", reason="검토", ip="1.2.3.4", ua="test",
            date=NOW, sign_data="x")

    ins("baro_sanction", id="1", user=USERS[4], kind="warn", scope="", reason="반복 오타",
        case_id="1", actor=USERS[0], start_date=NOW, end_date="", status="active", date=NOW)
    ins("baro_appeal", id="1", case_id="1", user=USERS[4], reason="억울합니다", extra="",
        status="pending", reviewer="", result="", result_reason="", date=NOW,
        deadline="2026-09-10", close_date="")
    ins("baro_quarantine", id="1", title=DOCS[5], rev="1", kind="copyright", reason="저작권 검토",
        actor=USERS[0], date=NOW, status="open", backup="원래 내용", release_date="", case_id="1")
    ins("baro_mass_op", id="1", kind="quarantine", actor=USERS[0], target_list="\n".join(DOCS),
        num="6", reason="일괄 검토", status="draft", approver="", created=NOW, approved_date="",
        execute_date="", grace_until="", urgent="", undo_status="", detail="")
    ins("baro_incident", id="1", title="검색 지연", status="open", severity="minor",
        detail="느립니다", date=NOW, resolve_date="", postmortem="", actor=USERS[0])
    ins("baro_award", id="1", season="2026", category="wikian", name="올해의 위키인",
        description="", status="open", date=NOW, result_reason="", winner="")
    ins("baro_nomination", id="1", award="1", target_kind="user", target=USERS[0],
        nominator=USERS[1], kind="user", score="0", status="open", date=NOW, reason="많이 기여했습니다")
    ins("baro_notify", id="1", user=USERS[0], kind="thanks", title="감사를 받았습니다",
        url="/baro/thanks", data="", readme="", date=NOW)
    ins("baro_workspace", id="1", project="sandbox", title="연습 초안", data="연습 중입니다",
        user=USERS[0], date=NOW, status="draft", note="")
    ins("baro_import", id="1", kind="json", source="친구 위키", file="", status="ready",
        actor=USERS[0], date=NOW, report="", num="3", license="CC BY-SA 4.0", options="{}")
    ins("baro_metric", id="1", key_name="edit", period=TODAY, value="12", detail="", date=NOW)
    ins("baro_backup", id="1", kind="daily", path="data/backup/x.db", size="1024", status="ok",
        date=NOW, note="", checksum="abc")
    ins("baro_privacy_req", id="1", user=USERS[0], kind="view", detail="열람 요청", status="new",
        handler="", date=NOW, close_date="", verify="on", result="")
    ins("baro_copyright_req", id="1", claimant="권리자", contact="a@b.c", target_kind="document",
        target=DOCS[0], proof="증빙", status="new", handler="", date=NOW, close_date="",
        result="", counter="")
    ins("baro_mentor", id="1", mentor=USERS[0], mentee="", status="active", date=NOW,
        field="history", note="도와드립니다")
    ins("baro_invite", id="1", code="ABC123XYZ890", owner=USERS[0], target="", date=NOW,
        used_date="", status="open")
    ins("baro_share_card", id="1", user=USERS[0], kind="doc", title="최동환 문서를 기록했습니다",
        data="{}", date=NOW, views="3")

    con.commit()
    print("시드 완료 : 사용자 %d명 · 문서 %d개 · 역할 %d개" % (len(USERS), len(DOCS), len(USERS)))


if __name__ == "__main__":
    main()
