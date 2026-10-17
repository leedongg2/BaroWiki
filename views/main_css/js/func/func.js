"use strict";

// ============================================================
//  자바스크립트 쪽 화면 문구 (한국어 · 영어 · 일본어)
//
//  <html lang="..."> 값을 따라간다. 없는 문구는 en-US 로 대신한다.
//  {n} 은 숫자로 바뀐다.
// ============================================================
var opennamu_lang_table = {
    "ko-KR" : {
        "ip_load_error" : "IP 정보를 불러오는 데 실패했습니다.",
        "time_now" : "방금 전",
        "time_second" : "{n}초 전",
        "time_minute" : "{n}분 전",
        "time_hour" : "{n}시간 전",
        "time_day" : "{n}일 전"
    },
    "en-US" : {
        "ip_load_error" : "Failed to load the IP information.",
        "time_now" : "just now",
        "time_second" : "{n} seconds ago",
        "time_minute" : "{n} minutes ago",
        "time_hour" : "{n} hours ago",
        "time_day" : "{n} days ago"
    },
    "ja-JP" : {
        "ip_load_error" : "IP情報の読み込みに失敗しました。",
        "time_now" : "たった今",
        "time_second" : "{n}秒前",
        "time_minute" : "{n}分前",
        "time_hour" : "{n}時間前",
        "time_day" : "{n}日前"
    }
};

function opennamu_lang_now() {
    var lang = (document.documentElement && document.documentElement.lang) || "";
    if(opennamu_lang_table[lang]) {
        return lang;
    }

    return "en-US";
}

function opennamu_lang(key, number) {
    var now = opennamu_lang_now();
    var text = null;

    if(opennamu_lang_table[now] && opennamu_lang_table[now][key] !== undefined) {
        text = opennamu_lang_table[now][key];
    } else if(opennamu_lang_table["en-US"][key] !== undefined) {
        text = opennamu_lang_table["en-US"][key];
    }

    if(text === null) {
        return key;
    }

    if(number !== undefined) {
        text = text.replace("{n}", String(number));
    }

    return text;
}


function opennamu_xss_filter(str) {
    return str.replace(/[&<>"']/g, function(match) {
        switch(match) {
            case '&':
                return '&amp;';
            case '<':
                return '&lt;';
            case '>':
                return '&gt;';
            case "'":
                return '&#x27;';
            case '"':
                return '&quot;';
        }
    });
}

function opennamu_do_ip_click(obj) {
    if (obj.id === "") {
        let user_name = obj.name;

        fetch('/api/v2/ip_menu/' + user_name).then(response => {
            if(!response.ok) {
                throw new Error(`API 호출 실패: ${response.status}`);
            }

            return response.json();
        }).then(data => {
            data = data["data"];

            let data_html = '';
            for(let key in data) {
                for(let for_a = 0; for_a < data[key].length; for_a++) {
                    data_html += '<a href="' + data[key][for_a][0] + '">' + data[key][for_a][1] + '</a> | ';
                }
            }

            data_html = data_html.replace(/ \| $/g, '');

            let for_a;
            for(for_a = 0; document.getElementById("opennamu_ip_render_" + String(for_a) + "_load"); for_a++) {}

            let popup_html = '<span class="opennamu_popup_footnote" id="opennamu_ip_render_' + String(for_a) + '_load" style="display: none;"></span>';
            popup_html += '<span style="display: none;" id="opennamu_ip_tool_' + String(for_a) + '">';
            popup_html += data_html;
            popup_html += '</span>';

            obj.innerHTML += popup_html;
            obj.id = 'opennamu_ip_render_' + String(for_a);
            obj.onclick = '';

            document.getElementById('opennamu_ip_render_' + String(for_a)).addEventListener("click", function () {
                opennamu_do_footnote_popover('opennamu_ip_render_' + String(for_a), '', 'opennamu_ip_tool_' + String(for_a), 'open');
            });
            document.addEventListener("click", function () {
                opennamu_do_footnote_popover('opennamu_ip_render_' + String(for_a), '', 'opennamu_ip_tool_' + String(for_a), 'close');
            });

            obj.click();
        }).catch(err => {
            console.error('IP 메뉴 호출 중 오류 발생:', err);
            obj.innerHTML = opennamu_lang('ip_load_error');
        });
    }
}

// ============================================================
//  분류 상자 · 이동 흔적 상자 펼치기
//
//  분류가 많으면 상자가 한 줄로 잘리면서 끝이 「…」 로 끝난다.
//  단추를 누르면 여러 줄로 펼쳐지고, 다시 누르면 접힌다.
//  한 줄에 다 들어가는 상자에서는 단추 자체를 숨긴다.
// ============================================================
function opennamu_do_box_spread(box_id) {
    var box = document.getElementById(box_id);
    if(box === null) {
        return;
    }

    var opened = box.classList.toggle('opennamu_box_open');
    var button = box.querySelector('.opennamu_category_button, .opennamu_trace_button');
    if(button !== null) {
        // 단추 글자는 서버에서 번역해 data 속성으로 실어 보낸다.
        // 옛 문서에 남아 있는 단추에는 속성이 없으므로 그때는 예전 모양을 쓴다.
        var more_text = button.getAttribute('data-more') || '(+)';
        var fold_text = button.getAttribute('data-fold') || '(-)';

        button.textContent = opened ? fold_text : more_text;
        button.title = opened ? fold_text : more_text;
        button.setAttribute('aria-expanded', opened ? 'true' : 'false');
    }
}

// 옛 이름 (문서 안에 남아 있을 수 있어 그대로 둔다)
function opennamu_do_trace_spread() {
    opennamu_do_box_spread('opennamu_trace');
}

function opennamu_do_category_spread() {
    opennamu_do_box_spread('cate');
}

function opennamu_box_button_check() {
    var list = document.querySelectorAll('.opennamu_category, .opennamu_trace');
    for(var for_a = 0; for_a < list.length; for_a++) {
        var box = list[for_a];
        var button = box.querySelector('.opennamu_category_button, .opennamu_trace_button');
        if(button === null) {
            continue;
        }

        // 이미 펼쳐 둔 상자는 건드리지 않는다
        if(box.classList.contains('opennamu_box_open')) {
            continue;
        }

        button.style.display = (box.scrollWidth > box.clientWidth + 1) ? '' : 'none';
    }
}

window.addEventListener('DOMContentLoaded', opennamu_box_button_check);
window.addEventListener('resize', opennamu_box_button_check);

function opennamu_list_hidden_remove() {
    const style = document.querySelector('#opennamu_list_hidden_style');
    if(style !== null) {
        if(style.innerHTML !== "") {
            style.innerHTML = '';
        } else {
            style.innerHTML = '.opennamu_list_hidden { display: none; }';
        }
    }
}

function opennamu_do_footnote_popover(set_name, load_name, sub_obj = undefined, do_type = 'open') {
    if(document.getElementById(set_name + '_load')) {
        if(do_type === 'open') {
            if(sub_obj !== undefined) {
                document.getElementById(set_name + '_load').innerHTML = document.getElementById(sub_obj).innerHTML;
            } else {
                document.getElementById(set_name).title = '';
                document.getElementById(set_name + '_load').innerHTML = '<a href="#' + load_name + '">(Go)</a> ' + document.getElementById(load_name + '_title').innerHTML;
            }
            document.getElementById(set_name + '_load').style.display = "inline-block";
            document.getElementById(set_name + '_load').count = 0;

            let width = document.getElementById(set_name + '_load').clientWidth;
            let screen_width = window.innerWidth;
            let left = document.getElementById(set_name).getBoundingClientRect().left;
            let top = window.pageYOffset + document.getElementById(set_name).getBoundingClientRect().top;

            document.getElementById(set_name + '_load').style.top = String(top) + "px";
            if(screen_width - (left + width) < 50) {
                if(left > 350) {
                    document.getElementById(set_name + '_load').style.left = String(left - 300) + "px";
                } else {
                    document.getElementById(set_name + '_load').style.left = "0px";
                }

                left = document.getElementById(set_name + '_load').getBoundingClientRect().left;
                width = document.getElementById(set_name + '_load').clientWidth;
                if(300 > width) {
                    document.getElementById(set_name + '_load').style.left = String(left + (300 - width)) + "px";
                } else {
                    document.getElementById(set_name + '_load').style.marginTop = "20px";
                }
            }
        } else {
            if(document.getElementById(set_name + '_load').count === 1) {
                document.getElementById(set_name + '_load').style.display = "none";
            } else {
                document.getElementById(set_name + '_load').count = 1;
            }
        }
    }
}

