"use strict";

let ringo_save_data = '';
let ringo_open = 0;
let ringo_menu_list = [
    'other_cel',
    'baro_cel',
    'user_cel',
    'add_cel'
];

function ringo_opening(data) {
    let element = [data];
    
    for(let for_a in ringo_menu_list) {
        if(ringo_menu_list[for_a] + '_in' !== data) {
            element.push(ringo_menu_list[for_a] + '_in');
        }
    }

    if((document.getElementById(element[0]).style.display === 'none' && ringo_open === 0) || ringo_save_data !== data) {
        document.getElementById(element[0]).style.display = 'block';

        for(let for_a in element) {
            if(for_a !== '0') { 
                if(document.getElementById(element[for_a]) !== null) {
                    document.getElementById(element[for_a]).style.display = 'none';
                }
            }
        }

        ringo_open = 1;
        ringo_save_data = data;

        setTimeout(function() { ringo_open = 2; }, 100);
    } else {
        document.getElementById(element[0]).style.display = 'none';

        ringo_open = 0
    }
}

document.addEventListener("click", function() {
    let cel_list = [];
    for(let for_a in ringo_menu_list) {
        cel_list.push(document.getElementById(ringo_menu_list[for_a]));
    }

    if(ringo_save_data !== '' && ringo_open === 2) {
        document.getElementById(ringo_save_data).style.display = 'none';

        setTimeout(function() { ringo_open = 0; }, 100);
    }
});

// 나무위키식 문단 접기 / 목차 접기
// DOM 구조를 바꾸지 않기 위해 래퍼에 display: contents 를 사용한다.
function ringo_do_fold_init() {
    const main = document.querySelector('.opennamu_main');
    if(main === null) {
        return;
    }

    // 목차 접기
    const toc = main.querySelector('.opennamu_TOC');
    if(toc !== null) {
        const toc_title = toc.querySelector('.opennamu_TOC_title');
        if(toc_title !== null) {
            const toggle = document.createElement('span');
            toggle.className = 'opennamu_TOC_toggle';
            toc_title.appendChild(toggle);
            toc_title.classList.add('opennamu_TOC_clickable');

            const toc_body = document.createElement('span');
            toc_body.className = 'opennamu_TOC_body';
            while(toc_title.nextSibling !== null) {
                toc_body.appendChild(toc_title.nextSibling);
            }
            toc.appendChild(toc_body);

            toc_title.addEventListener('click', function() {
                const folded = toc.classList.toggle('opennamu_TOC_folded');
                toc_body.classList.toggle('opennamu_TOC_body_folded', folded);
            });
        }
    }

    // 문단 접기
    const heading_list = main.querySelectorAll('h1, h2, h3, h4, h5, h6');
    for(let for_a = 0; for_a < heading_list.length; for_a++) {
        const heading = heading_list[for_a];

        // 나무마크가 만든 문단 제목(#toc 앵커를 가진 것)에만 접기를 적용한다.
        if(heading.classList.contains('blank') || heading.querySelector('a[href="#toc"]') === null) {
            continue;
        }

        const level = Number(heading.tagName.substring(1));
        const content = document.createElement('div');
        content.className = 'opennamu_heading_content';

        let node = heading.nextSibling;
        while(node !== null) {
            const next = node.nextSibling;
            if(node.nodeType === 1 && /^H[1-6]$/.test(node.tagName) && Number(node.tagName.substring(1)) <= level) {
                break;
            }

            content.appendChild(node);
            node = next;
        }

        if(content.childNodes.length === 0) {
            continue;
        }

        heading.parentNode.insertBefore(content, heading.nextSibling);
        heading.classList.add('opennamu_heading_fold');

        heading.addEventListener('click', function(event) {
            if(event.target.closest('a') !== null) {
                return;
            }

            const folded = heading.classList.toggle('opennamu_heading_folded');
            content.classList.toggle('opennamu_heading_content_folded', folded);
        });
    }
}

window.addEventListener('DOMContentLoaded', ringo_do_fold_init);