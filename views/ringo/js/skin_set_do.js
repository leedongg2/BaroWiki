"use strict";

function ringo_do_regex_data(data) {
    return new RegExp('(?:^|; )' + data + '=([^;]*)');
}

function ringo_do_skin_set() {
    let cookies = document.cookie;

    if(!window.localStorage.getItem('main_css_use_sys_darkmode') || window.localStorage.getItem('main_css_use_sys_darkmode') === '1') {
        if(cookies.match(ringo_do_regex_data('main_css_darkmode')) && cookies.match(ringo_do_regex_data('main_css_darkmode'))[1] === '1') {
            if(!window.matchMedia('(prefers-color-scheme: dark)').matches) {
                document.cookie = 'main_css_darkmode=0; path=/';
                history.go(0);
            }
        }

        if(!cookies.match(ringo_do_regex_data('main_css_darkmode')) || cookies.match(ringo_do_regex_data('main_css_darkmode'))[1] === '0') {
            if(window.matchMedia('(prefers-color-scheme: dark)').matches) {
                document.cookie = 'main_css_darkmode=1; path=/';
                history.go(0);
            }
        }
    }

    // 사이드바 끄기 (기본값 : 켜짐)
    if(window.localStorage.getItem('main_css_off_sidebar') === '1') {
        document.getElementById('ringo_add_style').innerHTML += `
            aside.do_fixed {
                display: none !important;
            }
        `;
    }

    // 고정폭
    if(window.localStorage.getItem('main_css_fixed_width') && window.localStorage.getItem('main_css_fixed_width') !== '') {
        let fixed_width_data = window.localStorage.getItem('main_css_fixed_width');
        document.getElementById('ringo_add_style').innerHTML += `
            .layout_container,
            .nav_bar_inner {
                max-width: ` + fixed_width_data + `px !important;
            }
        `;
    }

    // 사이드바 방향 (기본값 : 오른쪽, 나무위키와 동일)
    if(window.localStorage.getItem('main_css_sidebar_right') === '0') {
        document.getElementById('ringo_add_style').innerHTML += `
            @media screen and (min-width: 1024px) {
                .layout_row {
                    flex-direction: row-reverse !important;
                }
            }
        `;
    }
}

ringo_do_skin_set();
