from fontTools.ttLib import TTFont
from fontTools.ttLib.tables._m_a_x_p import table__m_a_x_p

# Load the font
font = TTFont("Go-Regular-2.otf")

# Create a new maxp table object
if "maxp" not in font:
    font["maxp"] = table__m_a_x_p("maxp")

# Recalculate the table based on other tables (like glyf)
font["maxp"].recalc(font)

# Save the new font
font.save("Go-Regular-3.otf")
