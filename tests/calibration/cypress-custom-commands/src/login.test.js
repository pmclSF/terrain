Cypress.Commands.add('loginAs', (username, password) => {
  cy.visit('/login');
  cy.get('#username').type(username);
  cy.get('#password').type(password);
  cy.get('#login-btn').click();
});

describe('login flow', () => {
  it('logs in as admin', () => {
    cy.loginAs('admin', 'pw');
    cy.url().should('include', '/dashboard');
  });

  it('logs in as guest', () => {
    cy.loginAs('guest', 'pw');
    cy.contains('Guest mode').should('be.visible');
  });
});
