CREATE TABLE admin_branches (
    admin_id INT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    branch_id INT NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    PRIMARY KEY (admin_id, branch_id)
);

