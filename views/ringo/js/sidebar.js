"use strict";

// func
function ringo_do_xss_encode(data) {
    data = String(data);

    data = data.replace(/'/g, '&#x27;');
    data = data.replace(/"/g, '&quot;');
    data = data.replace(/</g, '&lt;');
    data = data.replace(/>/g, '&gt;');

    return data;
}

function ringo_do_url_encode(data) {
    return encodeURIComponent(data);
}

// "n분 전" 형태로 표시한다. 문구는 위키 언어를 따라간다.
function ringo_do_lang(key, number) {
    if(typeof opennamu_lang === 'function') {
        return opennamu_lang(key, number);
    }

    // func.js 를 못 불러온 경우의 대비책
    const fallback = {
        'time_now' : 'just now',
        'time_second' : '{n} seconds ago',
        'time_minute' : '{n} minutes ago',
        'time_hour' : '{n} hours ago',
        'time_day' : '{n} days ago'
    };

    let text = fallback[key] || key;
    if(number !== undefined) {
        text = text.replace('{n}', String(number));
    }

    return text;
}

function ringo_do_time_ago(date_text) {
    const date = new Date(String(date_text).replace(/-/g, '/'));
    if(isNaN(date.getTime())) {
        return ringo_do_xss_encode(date_text);
    }

    const second = Math.floor((Date.now() - date.getTime()) / 1000);
    if(second < 0) {
        return ringo_do_lang('time_now');
    } else if(second < 60) {
        return ringo_do_lang('time_second', second);
    } else if(second < 3600) {
        return ringo_do_lang('time_minute', Math.floor(second / 60));
    } else if(second < 86400) {
        return ringo_do_lang('time_hour', Math.floor(second / 3600));
    } else if(second < 2592000) {
        return ringo_do_lang('time_day', Math.floor(second / 86400));
    }

    return ringo_do_xss_encode(String(date_text).substring(0, 10));
}

function ringo_do_side_item(href, title, date_text, sub_text) {
    const tip = ringo_do_xss_encode(date_text) + (sub_text ? ' | ' + ringo_do_xss_encode(sub_text) : '');

    return '' +
        '<div class="side_item" title="' + tip + '">' +
            '<a class="side_item_title" href="' + href + '">' + title + '</a>' +
            '<span class="side_item_meta">' + ringo_do_time_ago(date_text) + '</span>' +
        '</div>';
}

function ringo_do_side_select(number) {
    for(let for_a = 1; for_a <= 3; for_a++) {
        const button = document.getElementById('side_button_' + String(for_a));
        if(button) {
            if(for_a === number) {
                button.classList.add('selected');
            } else {
                button.classList.remove('selected');
            }
        }
    }
}

// event
function ringo_do_side_button_1() {
    ringo_do_side_select(1);

    if(temp_save[0] === '') {
        fetch("/api/recent_change/10").then(function(res) {
            return res.json();
        }).then(function(text) {
            let list = Array.isArray(text) ? text : text.data || [];
            let data = '';
            for(let for_a = 0; for_a < list.length; for_a++) {
                if(list[for_a][6] === '') {
                    data += ringo_do_side_item(
                        '/w/' + ringo_do_url_encode(list[for_a][1]),
                        ringo_do_xss_encode(list[for_a][1]),
                        list[for_a][2],
                        list[for_a][3]
                    );
                }
            }

            document.getElementById('side_content').innerHTML = data;
            temp_save[0] = data;
        }).catch(function(error) {
            document.getElementById('side_content').innerHTML = 'Error';
        });
    } else {
        document.getElementById('side_content').innerHTML = temp_save[0];
    }
}

function ringo_do_side_button_2() {
    ringo_do_side_select(2);

    if(temp_save[1] === '') {
        fetch("/api/recent_discuss/10").then(function(res) {
            return res.json();
        }).then(function(text) {
            let list = Array.isArray(text) ? text : text.data || [];
            let data = '';
            for(let for_a = 0; for_a < list.length; for_a++) {
                data += ringo_do_side_item(
                    '/thread/' + ringo_do_url_encode(list[for_a][3]),
                    ringo_do_xss_encode(list[for_a][1]),
                    list[for_a][2],
                    list[for_a][5]
                );
            }

            document.getElementById('side_content').innerHTML = data;
            temp_save[1] = data;
        }).catch(function(error) {
            document.getElementById('side_content').innerHTML = 'Error';
        });
    } else {
        document.getElementById('side_content').innerHTML = temp_save[1];
    }
}

function ringo_do_side_button_3() {
    ringo_do_side_select(3);

    if(temp_save[2] === '') {
        fetch("/api/v2/bbs/main").then(function(res) {
            return res.json();
        }).then(function(data) {
            let end_data = '';

            let text = data['data'];
            for(let for_a = 0; for_a < text.length; for_a++) {
                end_data += ringo_do_side_item(
                    '/bbs/w/' + text[for_a].set_id + '/' + text[for_a].set_code,
                    ringo_do_xss_encode(text[for_a].title),
                    text[for_a].date,
                    text[for_a].user_id
                );
            }

            document.getElementById('side_content').innerHTML = end_data;
            temp_save[2] = end_data;
        }).catch(function(error) {
            document.getElementById('side_content').innerHTML = 'Error';
        });
    } else {
        document.getElementById('side_content').innerHTML = temp_save[2];
    }
}

// init
let temp_save = ['', '', ''];

window.addEventListener('DOMContentLoaded', function() {
    if(document.getElementById("side_button_1")) {
        document.getElementById("side_button_1").addEventListener("click", ringo_do_side_button_1);
        document.getElementById("side_button_2").addEventListener("click", ringo_do_side_button_2);
        document.getElementById("side_button_3").addEventListener("click", ringo_do_side_button_3);

        if(window.localStorage.getItem('main_css_off_sidebar') !== '1') {
            ringo_do_side_button_1();
        }
    }
});
