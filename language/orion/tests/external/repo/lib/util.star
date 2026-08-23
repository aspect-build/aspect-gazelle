# Loaded from other files, itself loading a parent-directory file relatively.
load("./../helper.star", "EXTERNAL_SUFFIX")

def make_target_name(name):
    return name + "-" + EXTERNAL_SUFFIX
