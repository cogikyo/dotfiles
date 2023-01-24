import datetime

from kitty.fast_data_types import Screen
from kitty.tab_bar import DrawData, ExtraData, TabBarData, as_rgb, draw_title
from kitty.utils import color_as_int

center: str = ""

class DrawTab:
    """"""

    left_length: int = 0
    center_length: int = 0
    right_length: int = 0

    def __init__(self) -> None:
        self.right_length = len(self.get_datetime())

    def get_datetime(self) -> str:
        now = datetime.datetime.now()
        date = now.strftime(" %Y/%m/%d %H:%M:%S")
        return date

    def draw_left_status(self, screen: Screen, index: int) -> int:
        text = f"index: {index}"
        screen.draw(text)
        screen.cursor.x = len(text)
        self.left_length = len(text)
        return screen.cursor.x

    def draw_center_status(
        self,
        draw_data: DrawData,
        screen: Screen,
        tab: TabBarData,
        before: int,
        max_title_length: int,
        index: int,
        is_last: bool,
        extra_data: ExtraData,
    ) -> int:
        global center
        if center:
            center = center + " ┇ " + tab.title
        else:
            center = tab.title
        self.center_length = len(center)

        draw_spaces = screen.columns - screen.cursor.x - self.center_length
        if draw_spaces < 0:
            screen.draw(" " * draw_spaces)

        screen.cursor.x = screen.columns // 2 - self.center_length // 2
        screen.draw(center)
        return screen.cursor.x

    def draw_right_status(self, screen: Screen, is_last: bool) -> int:
        """draw right status"""
        end = screen.cursor.x
        if not is_last:
            return end

        draw_spaces = screen.columns - screen.cursor.x - self.right_length
        if draw_spaces > 0:
            screen.draw(" " * draw_spaces)
        screen.draw(self.get_datetime())
        if screen.columns - screen.cursor.x > self.right_length:
            screen.cursor.x = screen.columns - self.right_length
        return end

    def draw_tab(
        self,
        draw_data: DrawData,
        screen: Screen,
        tab: TabBarData,
        before: int,
        max_title_length: int,
        index: int,
        is_last: bool,
        extra_data: ExtraData,
    ) -> int:
        """"""
        global center
        if index == 1:
            center = ""
            self.draw_left_status(screen, index)

        self.draw_center_status(
            draw_data,
            screen,
            tab,
            before,
            max_title_length,
            index,
            is_last,
            extra_data,
        )

        if screen.cursor.x - (screen.columns - self.left_length - self.right_length - self.center_length) > 0:
            return screen.cursor.x

        self.draw_right_status(
            screen,
            is_last,
        )

        return screen.cursor.x


def draw_tab(*args) -> int:
    return DrawTab().draw_tab(*args)
